package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type HoldingLot struct {
	ID         string  `json:"id"`
	Type       string  `gorm:"size:10;default:''" json:"type,omitempty"`
	Date       int64   `json:"date"`
	Shares     float64 `json:"shares"`
	CostPrice  float64 `json:"costPrice,omitempty"`
	Cost       float64 `json:"cost,omitempty"`
	ValueAdded float64 `json:"valueAdded,omitempty"`
	Fee        float64 `json:"fee,omitempty"`
}

type JSONColumn []HoldingLot

func (j *JSONColumn) Scan(value interface{}) error {
	if value == nil {
		*j = JSONColumn{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("JSONColumn.Scan: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, j)
}

func (j JSONColumn) Value() (driver.Value, error) {
	if j == nil {
		return "[]", nil
	}
	return json.Marshal(j)
}

type AssetMapColumn map[string]float64

func (a *AssetMapColumn) Scan(value interface{}) error {
	if value == nil {
		*a = AssetMapColumn{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("AssetMapColumn.Scan: expected []byte, got %T", value)
	}
	return json.Unmarshal(bytes, a)
}

func (a AssetMapColumn) Value() (driver.Value, error) {
	if a == nil {
		return "{}", nil
	}
	return json.Marshal(a)
}

type Holding struct {
	ID        string     `gorm:"primaryKey" json:"id"`
	AssetId   string     `gorm:"index;size:20;not null" json:"assetId"`
	Symbol    string     `gorm:"size:20;default:''" json:"symbol"`
	Name      string     `gorm:"size:200;default:''" json:"name,omitempty"`
	Shares    float64    `gorm:"default:0" json:"shares"`
	Price     float64    `gorm:"default:0" json:"price"`
	CostPrice float64    `gorm:"default:0" json:"costPrice,omitempty"`
	Value     float64    `gorm:"default:0" json:"value"`
	Cost      float64    `gorm:"default:0" json:"cost,omitempty"`
	Date      int64      `gorm:"default:0" json:"date,omitempty"`
	Fee       float64    `gorm:"-" json:"fee,omitempty"`
	Lots      JSONColumn `gorm:"type:text;default:'[]'" json:"lots,omitempty"`
}

type PortfolioRecord struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	Timestamp int64          `gorm:"index;not null" json:"timestamp"`
	Assets    AssetMapColumn `gorm:"type:text;not null;default:'{}'" json:"assets"`
	Total     float64        `gorm:"default:0" json:"total"`
	Principal float64        `gorm:"default:0" json:"principal,omitempty"`
}

type Setting struct {
	Key   string `gorm:"primaryKey" json:"key"`
	Value string `gorm:"not null" json:"value"`
}
