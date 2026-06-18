package db

import (
	"permanent-portfolio/models"

	"github.com/libtnb/sqlite"
	"gorm.io/gorm"
)

func Init(dbPath string) (*gorm.DB, error) {
	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_txlock=immediate"
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

	if err := db.AutoMigrate(&models.Holding{}, &models.PortfolioRecord{}); err != nil {
		return nil, err
	}

	return db, nil
}
