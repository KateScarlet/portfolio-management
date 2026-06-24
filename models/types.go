package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
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

func (j *JSONColumn) Scan(value any) error {
	if value == nil {
		*j = JSONColumn{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("JSONColumn.Scan: expected []byte or string, got %T", value)
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

func (a *AssetMapColumn) Scan(value any) error {
	if value == nil {
		*a = AssetMapColumn{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("AssetMapColumn.Scan: expected []byte or string, got %T", value)
	}
	return json.Unmarshal(bytes, a)
}

func (a AssetMapColumn) Value() (driver.Value, error) {
	if a == nil {
		return "{}", nil
	}
	return json.Marshal(a)
}

type Portfolio struct {
	ID          string `gorm:"primaryKey" json:"id"`
	UserID      string `gorm:"index;not null" json:"userId"`
	Name        string `gorm:"size:100;not null" json:"name"`
	Description string `gorm:"size:500;default:''" json:"description,omitempty"`
	IsDefault   bool   `gorm:"default:false" json:"isDefault"`
	CreatedAt   int64  `json:"createdAt"`
}

type Holding struct {
	ID          string     `gorm:"primaryKey" json:"id"`
	UserID      string     `gorm:"index;not null" json:"userId"`
	PortfolioID string     `gorm:"index;not null" json:"portfolioId"`
	AssetId     string     `gorm:"size:20;not null" json:"assetId"`
	Symbol      string     `gorm:"size:20;default:''" json:"symbol"`
	Name        string     `gorm:"size:200;default:''" json:"name,omitempty"`
	Currency    string     `gorm:"size:10;default:'CNY'" json:"currency"`
	Shares      float64    `gorm:"default:0" json:"shares"`
	Price       float64    `gorm:"default:0" json:"price"`
	CostPrice   float64    `gorm:"default:0" json:"costPrice,omitempty"`
	Value       float64    `gorm:"default:0" json:"value"`
	Cost        float64    `gorm:"default:0" json:"cost,omitempty"`
	Date        int64      `gorm:"default:0" json:"date,omitempty"`
	Fee         float64    `gorm:"-" json:"fee,omitempty"`
	Lots        JSONColumn `gorm:"type:text;default:'[]'" json:"lots,omitempty"`
}

type HoldingSnapshot struct {
	AssetId   string  `json:"assetId"`
	Symbol    string  `json:"symbol"`
	Name      string  `json:"name"`
	Currency  string  `json:"currency"`
	Shares    float64 `json:"shares"`
	Price     float64 `json:"price"`
	CostPrice float64 `json:"costPrice"`
	Value     float64 `json:"value"`
	Cost      float64 `json:"cost"`
}

type HoldingSnapshotColumn []HoldingSnapshot

func (h *HoldingSnapshotColumn) Scan(value any) error {
	if value == nil {
		*h = HoldingSnapshotColumn{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("HoldingSnapshotColumn.Scan: expected []byte or string, got %T", value)
	}
	return json.Unmarshal(bytes, h)
}

func (h HoldingSnapshotColumn) Value() (driver.Value, error) {
	if h == nil {
		return "[]", nil
	}
	return json.Marshal(h)
}

type PortfolioRecord struct {
	ID          string                `gorm:"primaryKey" json:"id"`
	UserID      string                `gorm:"index;not null" json:"userId"`
	PortfolioID string                `gorm:"index;not null" json:"portfolioId"`
	Timestamp   int64                 `gorm:"index;not null" json:"timestamp"`
	Assets      AssetMapColumn        `gorm:"type:text;not null;default:'{}'" json:"assets"`
	Holdings    HoldingSnapshotColumn `gorm:"type:text;not null;default:'[]'" json:"holdings"`
	Total       float64               `gorm:"default:0" json:"total"`
	Principal   float64               `gorm:"default:0" json:"principal"`
}

type Setting struct {
	Key         string `gorm:"primaryKey;size:100" json:"key"`
	Value       string `gorm:"not null" json:"value"`
	UserID      string `gorm:"primaryKey;size:50;default:''" json:"userId,omitempty"`
	PortfolioID string `gorm:"primaryKey;size:50;default:''" json:"portfolioId,omitempty"`
}

type AvailableFund struct {
	ID          string  `gorm:"primaryKey" json:"id"`
	UserID      string  `gorm:"index;not null" json:"userId"`
	PortfolioID string  `gorm:"index;not null" json:"portfolioId"`
	Currency    string  `gorm:"size:10;not null" json:"currency"`
	Amount      float64 `gorm:"default:0" json:"amount"`
}

type FundTransaction struct {
	ID                string  `gorm:"primaryKey" json:"id"`
	UserID            string  `gorm:"index;not null" json:"userId"`
	PortfolioID       string  `gorm:"index;not null" json:"portfolioId"`
	Type              string  `gorm:"size:20;not null" json:"type"`
	Amount            float64 `gorm:"not null" json:"amount"`
	Currency          string  `gorm:"size:10;not null" json:"currency"`
	TargetPortfolioID string  `gorm:"size:50;default:''" json:"targetPortfolioId,omitempty"`
	TargetAmount      float64 `gorm:"default:0" json:"targetAmount,omitempty"`
	TargetCurrency    string  `gorm:"size:10;default:''" json:"targetCurrency,omitempty"`
	ExchangeRate      float64 `gorm:"default:0" json:"exchangeRate,omitempty"`
	HoldingID         string  `gorm:"size:50;default:''" json:"holdingId,omitempty"`
	Note              string  `gorm:"size:500;default:''" json:"note,omitempty"`
	CreatedAt         int64   `json:"createdAt"`
}

type User struct {
	ID          string `gorm:"primaryKey" json:"id"`
	Username    string `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Password    string `gorm:"size:200;default:''" json:"-"`
	Role        string `gorm:"size:20;default:'user'" json:"role"`
	SSOProvider string `gorm:"size:20;default:''" json:"ssoProvider,omitempty"`
	SSOId       string `gorm:"size:200;default:''" json:"-"`
	CreatedAt   int64  `json:"createdAt"`
}

type WebAuthnCredential struct {
	ID           string `gorm:"primaryKey" json:"id"`
	UserID       string `gorm:"index;not null" json:"userId"`
	Name         string `gorm:"size:100;default:''" json:"name"`
	CredentialID []byte `gorm:"type:bytes;not null" json:"-"`
	PublicKey    []byte `gorm:"type:bytes;not null" json:"-"`
	Flags        uint8  `gorm:"default:0" json:"-"`
	SignCount    uint64 `gorm:"default:0" json:"-"`
	CreatedAt    int64  `json:"createdAt"`
	LastUsedAt   int64  `json:"lastUsedAt"`
}

type WebAuthnSession struct {
	ID        string `gorm:"primaryKey"`
	Data      string `gorm:"type:text;not null"`
	ExpiresAt int64  `gorm:"not null"`
}

// RecalcFromLots recalculates holding fields from its lots.
// This is the single source of truth for financial calculations.
//
// Convention:
//   - Buy lot: Cost = raw cost (shares * costPrice, NO fee); ValueAdded = market value at purchase; Fee = transaction fee
//   - Sell lot: Cost = proportional cost reduction; ValueAdded = value removed from holding; Fee = transaction fee
//   - Holding: Cost = total buy costs - total sell costs (NO fees); Value = current market value
//   - Total investment (principal) = Cost + BuyFees()
func (h *Holding) RecalcFromLots() {
	if len(h.Lots) == 0 {
		return
	}

	var totalBuyShares, totalSellShares float64
	var totalBuyCost, totalSellCost float64
	var totalBuyValue, totalSellValue float64

	for _, lot := range h.Lots {
		if lot.Type == "sell" {
			totalSellShares += lot.Shares
			totalSellCost += lot.Cost
			totalSellValue += lot.ValueAdded
		} else {
			totalBuyShares += lot.Shares
			totalBuyCost += lot.Cost
			totalBuyValue += lot.ValueAdded
		}
	}

	if h.Symbol != "" {
		h.Shares = totalBuyShares - totalSellShares
		h.Cost = totalBuyCost - totalSellCost
		if math.Abs(h.Shares) < 1e-9 {
			h.Shares = 0
		}
		if h.Shares > 0 {
			h.CostPrice = h.Cost / h.Shares
		} else {
			h.CostPrice = 0
		}
		h.Value = h.Shares * h.Price
	} else {
		h.Shares = totalBuyShares - totalSellShares
		h.Value = totalBuyValue - totalSellValue
		h.Cost = totalBuyCost - totalSellCost
		if math.Abs(h.Shares) < 1e-9 {
			h.Shares = 0
		}
		if h.Shares > 0 {
			h.CostPrice = h.Cost / h.Shares
		} else {
			h.CostPrice = 0
		}
	}
}

// TotalFees returns the sum of all lot fees for this holding.
func (h *Holding) TotalFees() float64 {
	var total float64
	for _, lot := range h.Lots {
		total += lot.Fee
	}
	return total
}

// BuyFees returns the sum of buy lot fees only (excludes sell lot fees).
// Sell fees are already deducted from realizedValue, so including them
// in principal would double-count the cost.
func (h *Holding) BuyFees() float64 {
	var total float64
	for _, lot := range h.Lots {
		if lot.Type != "sell" {
			total += lot.Fee
		}
	}
	return total
}
