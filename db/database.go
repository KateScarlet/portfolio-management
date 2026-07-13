package db

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"portfolio-management/models"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var baseDir string

func BaseDir() string {
	if baseDir != "" {
		return baseDir
	}
	execPath, err := os.Executable()
	if err != nil {
		panic("failed to get executable path: " + err.Error())
	}
	return filepath.Dir(execPath)
}

// SetBaseDir overrides the base directory (for testing)
func SetBaseDir(dir string) {
	baseDir = dir
}

func ConfigDir() string {
	return filepath.Join(BaseDir(), "config")
}

func ConfigFile() string {
	return filepath.Join(BaseDir(), "config", "config.yaml")
}

type Config struct {
	JWTSecret    string `mapstructure:"jwtSecret"` //nolint:gosec // Config field, not exposed
	CookieSecure bool   `mapstructure:"cookieSecure"`
	Database     struct {
		Type string `mapstructure:"type"`
		DSN  string `mapstructure:"dsn"`
	} `mapstructure:"database"`
	OIDC struct {
		Enabled      bool   `mapstructure:"enabled"`
		Issuer       string `mapstructure:"issuer"`
		ClientID     string `mapstructure:"clientID"`
		ClientSecret string `mapstructure:"clientSecret"` //nolint:gosec // Config field, not exposed
		RedirectURL  string `mapstructure:"redirectURL"`
	} `mapstructure:"oidc"`
	WebAuthn struct {
		Enabled   bool     `mapstructure:"enabled"`
		RPID      string   `mapstructure:"rpid"`
		RPOrigins []string `mapstructure:"rpOrigins"`
	} `mapstructure:"webauthn"`
}

func LoadConfig() *Config {
	v := viper.GetViper()

	v.SetConfigFile(ConfigFile())
	v.SetConfigType("yaml")

	v.SetDefault("database.type", "postgres")
	v.SetDefault("database.dsn", "postgres://localhost:5432/portfolio?sslmode=disable")

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			if os.IsNotExist(err) {
				return &Config{}
			}
			panic("failed to read config: " + err.Error())
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic("failed to unmarshal config: " + err.Error())
	}

	if cfg.JWTSecret == "" {
		cfg.JWTSecret = generateJWTSecret()
		if err := SaveConfig(&cfg); err != nil {
			panic("failed to save config: " + err.Error())
		}
	}

	return &cfg
}

func generateJWTSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate JWT secret: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func Init(cfg *Config) (*gorm.DB, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	dbType := cfg.Database.Type
	if dbType == "" {
		dbType = "postgres"
	}
	dsn := cfg.Database.DSN
	if dsn == "" {
		dsn = "postgres://localhost:5432/portfolio?sslmode=disable"
	}

	if dbType != "postgres" {
		return nil, fmt.Errorf("unsupported database type: %s (only postgres is supported)", dbType)
	}

	return initPostgres(dsn)
}

func initPostgres(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(30)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err := removeLegacyDividendLedger(db); err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&models.Portfolio{}, &models.Holding{}, &models.HoldingLot{}, &models.PortfolioRecord{}, &models.Setting{}, &models.User{}, &models.WebAuthnCredential{}, &models.WebAuthnSession{}, &models.AvailableFund{}, &models.FundTransaction{}, &models.Account{}, &models.Dividend{}); err != nil {
		return nil, err
	}

	// 迁移：将 account_id 为 NULL 的持仓转移到默认账户
	var holdingsWithNull []models.Holding
	if err := db.Where("account_id IS NULL").Find(&holdingsWithNull).Error; err == nil && len(holdingsWithNull) > 0 {
		userIDs := make(map[uuid.UUID]bool)
		for i := range holdingsWithNull {
			userIDs[holdingsWithNull[i].UserID] = true
		}
		for userID := range userIDs {
			var defaultAccount models.Account
			if err := db.Where("user_id = ? AND is_default = ?", userID, true).First(&defaultAccount).Error; err != nil {
				continue
			}
			db.Model(&models.Holding{}).Where("user_id = ? AND account_id IS NULL", userID).Update("account_id", defaultAccount.ID)
		}
	}
	// 将 account_id 列改为 NOT NULL
	db.Exec("ALTER TABLE holdings ALTER COLUMN account_id SET NOT NULL")

	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_holdings_portfolio_symbol_account ON holdings(portfolio_id, symbol, account_id) WHERE symbol != ''")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_holdings_portfolio_name_asset_account ON holdings(portfolio_id, name, asset_id, account_id) WHERE symbol = ''")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_settings_user_id ON settings(user_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_settings_portfolio_id ON settings(portfolio_id)")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_users_sso ON users(sso_provider, sso_id) WHERE sso_provider != ''")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_records_portfolio_ts ON portfolio_records(portfolio_id, timestamp DESC)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_webauthn_sessions_expires ON webauthn_sessions(expires_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_webauthn_creds_cred_id ON webauthn_credentials(credential_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_holdings_portfolio_asset ON holdings(portfolio_id, asset_id)")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_available_funds_unique ON available_funds(user_id, portfolio_id, currency)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_fund_transactions_portfolio_ts ON fund_transactions(portfolio_id, created_at DESC)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_dividend_events_holding_date ON dividend_events(holding_id, payment_date DESC)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_holdings_account_id ON holdings(account_id)")

	return db, nil
}

// removeLegacyDividendLedger is a one-time destructive migration. The old
// dividends table used a different accounting contract, so its records cannot
// be represented safely in dividend_events. Linked DRIP lots and transaction
// log rows are removed; existing cash balances are retained as opening cash.
func removeLegacyDividendLedger(db *gorm.DB) error {
	if !db.Migrator().HasTable("dividends") {
		return nil
	}
	type legacyDividend struct {
		HoldingID    uuid.UUID
		HoldingLotID *uuid.UUID
		FundTxID     *uuid.UUID
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var rows []legacyDividend
		if err := tx.Table("dividends").Select("holding_id", "holding_lot_id", "fund_tx_id").Find(&rows).Error; err != nil {
			return err
		}
		holdingIDs := make([]uuid.UUID, 0, len(rows))
		seen := make(map[uuid.UUID]struct{}, len(rows))
		for _, row := range rows {
			if row.HoldingLotID != nil {
				if err := tx.Where("id = ?", *row.HoldingLotID).Delete(&models.HoldingLot{}).Error; err != nil {
					return err
				}
			}
			if row.FundTxID != nil {
				if err := tx.Where("id = ?", *row.FundTxID).Delete(&models.FundTransaction{}).Error; err != nil {
					return err
				}
			}
			if _, ok := seen[row.HoldingID]; !ok {
				seen[row.HoldingID] = struct{}{}
				holdingIDs = append(holdingIDs, row.HoldingID)
			}
		}
		for _, holdingID := range holdingIDs {
			var holding models.Holding
			if err := tx.First(&holding, "id = ?", holdingID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}
			lots, err := models.LoadLots(tx, holdingID)
			if err != nil {
				return err
			}
			models.RecalcFromLots(&holding, lots)
			if err := tx.Model(&holding).Updates(map[string]any{
				"shares": holding.Shares, "value": holding.Value, "cost": holding.Cost,
				"cost_price": holding.CostPrice, "total_dividends": 0,
			}).Error; err != nil {
				return err
			}
		}
		return tx.Migrator().DropTable("dividends")
	})
}

func IsSetupMode() bool {
	_, err := os.Stat(ConfigFile())
	return os.IsNotExist(err)
}

// SaveConfig writes the configuration to config file
func SaveConfig(cfg *Config) error {
	if err := os.MkdirAll(ConfigDir(), 0o750); err != nil {
		return err
	}

	v := viper.New()
	v.Set("jwtSecret", cfg.JWTSecret)
	v.Set("database.type", cfg.Database.Type)
	v.Set("database.dsn", cfg.Database.DSN)
	v.Set("oidc.enabled", cfg.OIDC.Enabled)
	v.Set("oidc.issuer", cfg.OIDC.Issuer)
	v.Set("oidc.clientID", cfg.OIDC.ClientID)
	v.Set("oidc.clientSecret", cfg.OIDC.ClientSecret)
	v.Set("oidc.redirectURL", cfg.OIDC.RedirectURL)
	v.Set("webauthn.enabled", cfg.WebAuthn.Enabled)
	v.Set("webauthn.rpid", cfg.WebAuthn.RPID)
	v.Set("webauthn.rpOrigins", cfg.WebAuthn.RPOrigins)
	v.SetConfigFile(ConfigFile())
	v.SetConfigType("yaml")
	return v.WriteConfig()
}
