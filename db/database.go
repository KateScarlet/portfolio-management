package db

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"portfolio-management/models"
	"time"

	"github.com/libtnb/sqlite"
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

	v.SetDefault("database.type", "sqlite")
	v.SetDefault("database.dsn", filepath.Join(BaseDir(), "data", "portfolio.db"))

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
		dbType = "sqlite"
	}
	dsn := cfg.Database.DSN
	if dsn == "" {
		if dbType == "postgres" {
			dsn = "postgres://localhost:5432/portfolio?sslmode=disable"
		} else {
			dsn = filepath.Join(BaseDir(), "data", "portfolio.db")
		}
	} else if dbType == "sqlite" && !filepath.IsAbs(dsn) {
		dsn = filepath.Join(BaseDir(), dsn)
	}

	switch dbType {
	case "postgres":
		return initPostgres(dsn)
	default:
		return initSQLite(dsn)
	}
}

func initSQLite(dsn string) (*gorm.DB, error) {
	if dir := filepath.Dir(dsn); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(5)
	sqlDB.SetMaxIdleConns(5)

	db.Exec("PRAGMA journal_mode=WAL")

	if err := db.AutoMigrate(&models.Portfolio{}, &models.Holding{}, &models.PortfolioRecord{}, &models.Setting{}, &models.User{}, &models.WebAuthnCredential{}, &models.WebAuthnSession{}, &models.AvailableFund{}, &models.FundTransaction{}); err != nil {
		return nil, err
	}

	migrateToMultiPortfolio(db)
	migrateAvailableFunds(db)

	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_holdings_portfolio_symbol ON holdings(portfolio_id, symbol) WHERE symbol != ''")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_holdings_portfolio_name_asset ON holdings(portfolio_id, name, asset_id) WHERE symbol = ''")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_settings_user_id ON settings(user_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_settings_portfolio_id ON settings(portfolio_id)")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_users_sso ON users(sso_provider, sso_id) WHERE sso_provider != ''")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_records_portfolio_ts ON portfolio_records(portfolio_id, timestamp DESC)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_webauthn_sessions_expires ON webauthn_sessions(expires_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_webauthn_creds_cred_id ON webauthn_credentials(credential_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_holdings_portfolio_asset ON holdings(portfolio_id, asset_id)")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_available_funds_unique ON available_funds(user_id, portfolio_id, currency)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_fund_transactions_portfolio_ts ON fund_transactions(portfolio_id, created_at DESC)")
	db.Exec("DELETE FROM holdings WHERE user_id = '' OR user_id IS NULL")
	db.Exec("DELETE FROM portfolio_records WHERE user_id = '' OR user_id IS NULL")
	db.Exec("DELETE FROM settings WHERE user_id IS NULL")

	return db, nil
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
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	if err := db.AutoMigrate(&models.Portfolio{}, &models.Holding{}, &models.PortfolioRecord{}, &models.Setting{}, &models.User{}, &models.WebAuthnCredential{}, &models.WebAuthnSession{}, &models.AvailableFund{}, &models.FundTransaction{}); err != nil {
		return nil, err
	}

	migrateToMultiPortfolio(db)
	migrateAvailableFunds(db)

	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_holdings_portfolio_symbol ON holdings(portfolio_id, symbol) WHERE symbol != ''")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_holdings_portfolio_name_asset ON holdings(portfolio_id, name, asset_id) WHERE symbol = ''")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_settings_user_id ON settings(user_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_settings_portfolio_id ON settings(portfolio_id)")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_users_sso ON users(sso_provider, sso_id) WHERE sso_provider != ''")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_records_portfolio_ts ON portfolio_records(portfolio_id, timestamp DESC)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_webauthn_sessions_expires ON webauthn_sessions(expires_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_webauthn_creds_cred_id ON webauthn_credentials(credential_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_holdings_portfolio_asset ON holdings(portfolio_id, asset_id)")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_available_funds_unique ON available_funds(user_id, portfolio_id, currency)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_fund_transactions_portfolio_ts ON fund_transactions(portfolio_id, created_at DESC)")
	db.Exec("DELETE FROM holdings WHERE user_id = '' OR user_id IS NULL")
	db.Exec("DELETE FROM portfolio_records WHERE user_id = '' OR user_id IS NULL")
	db.Exec("DELETE FROM settings WHERE user_id IS NULL")

	return db, nil
}

// migrateToMultiPortfolio creates a default portfolio for existing users
// and links their holdings, records, and settings to it.
func migrateToMultiPortfolio(db *gorm.DB) {
	var count int64
	db.Model(&models.Holding{}).Where("portfolio_id = '' OR portfolio_id IS NULL").Count(&count)
	if count == 0 {
		return
	}

	slog.Info("migrating to multi-portfolio: found holdings without portfolio_id", "count", count)

	var userIDs []string
	db.Model(&models.Holding{}).Where("portfolio_id = '' OR portfolio_id IS NULL").Distinct("user_id").Pluck("user_id", &userIDs)

	for _, userID := range userIDs {
		portfolioID := generateID()

		portfolio := models.Portfolio{
			ID:        portfolioID,
			UserID:    userID,
			Name:      "默认组合",
			IsDefault: true,
			CreatedAt: time.Now().UnixMilli(),
		}
		if err := db.Create(&portfolio).Error; err != nil {
			slog.Error("failed to create default portfolio during migration", "userId", userID, "error", err)
			continue
		}

		db.Model(&models.Holding{}).Where("user_id = ? AND (portfolio_id = '' OR portfolio_id IS NULL)", userID).Update("portfolio_id", portfolioID)
		db.Model(&models.PortfolioRecord{}).Where("user_id = ? AND (portfolio_id = '' OR portfolio_id IS NULL)", userID).Update("portfolio_id", portfolioID)
		db.Model(&models.Setting{}).Where("user_id = ? AND (portfolio_id = '' OR portfolio_id IS NULL)", userID).Update("portfolio_id", portfolioID)

		slog.Info("migrated user to default portfolio", "userId", userID, "portfolioId", portfolioID)
	}
}

// migrateAvailableFunds moves the old single availableFunds setting
// into the new available_funds table with currency=CNY.
func migrateAvailableFunds(db *gorm.DB) {
	var settings []models.Setting
	if err := db.Where("`key` = ? AND portfolio_id != ''", "availableFunds").Find(&settings).Error; err != nil {
		return
	}
	if len(settings) == 0 {
		return
	}
	slog.Info("migrating availableFunds to available_funds table", "count", len(settings))
	for _, s := range settings {
		var amount float64
		if _, err := fmt.Sscanf(s.Value, "%f", &amount); err != nil {
			continue
		}
		af := models.AvailableFund{
			ID:          generateID(),
			UserID:      s.UserID,
			PortfolioID: s.PortfolioID,
			Currency:    "CNY",
			Amount:      amount,
		}
		db.Where(models.AvailableFund{UserID: s.UserID, PortfolioID: s.PortfolioID, Currency: "CNY"}).
			Assign(models.AvailableFund{Amount: amount}).
			FirstOrCreate(&af)
		db.Delete(&models.Setting{}, "`key` = ? AND user_id = ? AND portfolio_id = ?", "availableFunds", s.UserID, s.PortfolioID)
	}
	slog.Info("availableFunds migration complete")
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate ID: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// IsSetupMode checks if config file exists
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
