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

	// Migrate created_at columns from bigint (millisecond timestamps) to timestamptz.
	// Must run before AutoMigrate, which cannot auto-convert bigint→timestamptz.
	migrateCreatedAtColumns(db)

	if err := db.AutoMigrate(&models.Portfolio{}, &models.Holding{}, &models.HoldingLot{}, &models.PortfolioRecord{}, &models.Setting{}, &models.User{}, &models.WebAuthnCredential{}, &models.WebAuthnSession{}, &models.AvailableFund{}, &models.FundTransaction{}, &models.Account{}, &models.Dividend{}); err != nil {
		return nil, err
	}

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
	db.Exec("CREATE INDEX IF NOT EXISTS idx_holdings_account_id ON holdings(account_id)")

	return db, nil
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

// migrateCreatedAtColumns converts created_at columns from bigint (millisecond
// timestamps) to timestamptz. It checks each table's column type before attempting
// migration so it's safe to run repeatedly.
func migrateCreatedAtColumns(db *gorm.DB) {
	tables := []string{"portfolios", "accounts", "fund_transactions", "users", "web_authn_credentials", "dividends"}
	for _, table := range tables {
		var dataType string
		db.Raw("SELECT data_type FROM information_schema.columns WHERE table_name = ? AND column_name = 'created_at'", table).Scan(&dataType)
		if dataType == "bigint" || dataType == "int8" {
			db.Exec(fmt.Sprintf("ALTER TABLE %s ALTER COLUMN created_at TYPE timestamptz USING to_timestamp(created_at / 1000.0)", table))
		}
	}
}
