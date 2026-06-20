package db

import (
	"permanent-portfolio/models"
	"time"

	"github.com/libtnb/sqlite"
	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Config struct {
	Database struct {
		Type string `mapstructure:"type"`
		DSN  string `mapstructure:"dsn"`
	} `mapstructure:"database"`
}

func LoadConfig() *Config {
	v := viper.GetViper()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

	v.SetDefault("database.type", "sqlite")
	v.SetDefault("database.dsn", "portfolio.db")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic("failed to read config: " + err.Error())
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic("failed to unmarshal config: " + err.Error())
	}

	return &cfg
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

	if err := db.AutoMigrate(&models.Holding{}, &models.PortfolioRecord{}, &models.Setting{}); err != nil {
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

	if err := db.AutoMigrate(&models.Holding{}, &models.PortfolioRecord{}, &models.Setting{}); err != nil {
		return nil, err
	}

	return db, nil
}
