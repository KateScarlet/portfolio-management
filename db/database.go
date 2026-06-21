package db

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"portfolio-management/models"
	"time"

	"github.com/libtnb/sqlite"
	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const ConfigDir = "config"
const ConfigFile = "config/config.yaml"

type Config struct {
	Database struct {
		Type string `mapstructure:"type"`
		DSN  string `mapstructure:"dsn"`
	} `mapstructure:"database"`
	OIDC struct {
		Enabled      bool   `mapstructure:"enabled"`
		Issuer       string `mapstructure:"issuer"`
		ClientID     string `mapstructure:"clientID"`
		ClientSecret string `mapstructure:"clientSecret"`
		RedirectURL  string `mapstructure:"redirectURL"`
	} `mapstructure:"oidc"`
}

func LoadConfig() *Config {
	v := viper.GetViper()

	v.SetConfigFile(ConfigFile)
	v.SetConfigType("yaml")

	v.SetDefault("database.type", "sqlite")
	v.SetDefault("database.dsn", "portfolio.db")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
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

	return &cfg
}

func GenerateJWTSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
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
			dsn = "portfolio.db"
		}
	}

	switch dbType {
	case "postgres":
		return initPostgres(dsn)
	default:
		return initSQLite(dsn)
	}
}

func initSQLite(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := db.AutoMigrate(&models.Holding{}, &models.PortfolioRecord{}, &models.Setting{}, &models.User{}); err != nil {
		return nil, err
	}

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

	if err := db.AutoMigrate(&models.Holding{}, &models.PortfolioRecord{}, &models.Setting{}, &models.User{}); err != nil {
		return nil, err
	}

	return db, nil
}

// IsSetupMode checks if config file exists
func IsSetupMode() bool {
	_, err := os.Stat(ConfigFile)
	return os.IsNotExist(err)
}

// SaveConfig writes the configuration to config file
func SaveConfig(cfg *Config) error {
	if err := os.MkdirAll(ConfigDir, 0755); err != nil {
		return err
	}

	v := viper.New()
	v.Set("database.type", cfg.Database.Type)
	v.Set("database.dsn", cfg.Database.DSN)
	v.Set("oidc.enabled", cfg.OIDC.Enabled)
	v.Set("oidc.issuer", cfg.OIDC.Issuer)
	v.Set("oidc.clientID", cfg.OIDC.ClientID)
	v.Set("oidc.clientSecret", cfg.OIDC.ClientSecret)
	v.Set("oidc.redirectURL", cfg.OIDC.RedirectURL)
	v.SetConfigFile(ConfigFile)
	v.SetConfigType("yaml")
	return v.WriteConfig()
}
