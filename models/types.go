package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
)

type HoldingLot struct {
	ID         string          `json:"id"`
	Type       string          `gorm:"size:10;default:''" json:"type,omitempty"`
	Date       int64           `json:"date"`
	Shares     decimal.Decimal `json:"shares"`
	CostPrice  decimal.Decimal `json:"costPrice"`
	Cost       decimal.Decimal `json:"cost"`
	ValueAdded decimal.Decimal `json:"valueAdded"`
	Fee        decimal.Decimal `json:"fee"`
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

type AssetMapColumn map[string]decimal.Decimal

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

type Account struct {
	ID          string `gorm:"primaryKey" json:"id"`
	UserID      string `gorm:"index;not null" json:"userId"`
	Name        string `gorm:"size:100;not null" json:"name"`
	Description string `gorm:"size:500;default:''" json:"description,omitempty"`
	Broker      string `gorm:"size:100;default:''" json:"broker,omitempty"`
	CreatedAt   int64  `json:"createdAt"`
}

type Holding struct {
	ID          string          `gorm:"primaryKey" json:"id"`
	UserID      string          `gorm:"index;not null" json:"userId"`
	PortfolioID string          `gorm:"index;not null" json:"portfolioId"`
	AccountID   string          `gorm:"size:50;default:'';index" json:"accountId,omitempty"`
	AssetId     string          `gorm:"size:20;not null" json:"assetId"`
	Symbol      string          `gorm:"size:20;default:''" json:"symbol"`
	Name        string          `gorm:"size:200;default:''" json:"name,omitempty"`
	Market      string          `gorm:"size:20;default:''" json:"market,omitempty"`
	Currency    string          `gorm:"size:10;default:'CNY'" json:"currency"`
	Shares      decimal.Decimal `gorm:"default:0" json:"shares"`
	Price       decimal.Decimal `gorm:"default:0" json:"price"`
	CostPrice   decimal.Decimal `gorm:"default:0" json:"costPrice"`
	Value       decimal.Decimal `gorm:"default:0" json:"value"`
	Cost        decimal.Decimal `gorm:"default:0" json:"cost"`
	Date        int64           `gorm:"default:0" json:"date,omitempty"`
	Fee         decimal.Decimal `gorm:"-" json:"fee"`
	Lots        JSONColumn      `gorm:"type:text;default:'[]'" json:"lots,omitempty"`
}

type HoldingSnapshot struct {
	AssetId   string          `json:"assetId"`
	Symbol    string          `json:"symbol"`
	Name      string          `json:"name"`
	Currency  string          `json:"currency"`
	Shares    decimal.Decimal `json:"shares"`
	Price     decimal.Decimal `json:"price"`
	CostPrice decimal.Decimal `json:"costPrice"`
	Value     decimal.Decimal `json:"value"`
	Cost      decimal.Decimal `json:"cost"`
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
	Total       decimal.Decimal       `gorm:"default:0" json:"total"`
	Principal   decimal.Decimal       `gorm:"default:0" json:"principal"`
}

type Setting struct {
	Key         string `gorm:"primaryKey;size:100" json:"key"`
	Value       string `gorm:"not null" json:"value"`
	UserID      string `gorm:"primaryKey;size:50;default:''" json:"userId,omitempty"`
	PortfolioID string `gorm:"primaryKey;size:50;default:''" json:"portfolioId,omitempty"`
}

type AvailableFund struct {
	ID          string          `gorm:"primaryKey" json:"id"`
	UserID      string          `gorm:"index;not null" json:"userId"`
	PortfolioID string          `gorm:"index;not null" json:"portfolioId"`
	Currency    string          `gorm:"size:10;not null" json:"currency"`
	Amount      decimal.Decimal `gorm:"default:0" json:"amount"`
}

type FundTransaction struct {
	ID                string          `gorm:"primaryKey" json:"id"`
	UserID            string          `gorm:"index;not null" json:"userId"`
	PortfolioID       string          `gorm:"index;not null" json:"portfolioId"`
	Type              string          `gorm:"size:20;not null" json:"type"`
	Amount            decimal.Decimal `gorm:"not null" json:"amount"`
	Currency          string          `gorm:"size:10;not null" json:"currency"`
	TargetPortfolioID string          `gorm:"size:50;default:''" json:"targetPortfolioId,omitempty"`
	TargetAmount      decimal.Decimal `gorm:"default:0" json:"targetAmount"`
	TargetCurrency    string          `gorm:"size:10;default:''" json:"targetCurrency,omitempty"`
	ExchangeRate      decimal.Decimal `gorm:"default:0" json:"exchangeRate"`
	HoldingID         string          `gorm:"size:50;default:''" json:"holdingId,omitempty"`
	Note              string          `gorm:"size:500;default:''" json:"note,omitempty"`
	CreatedAt         int64           `json:"createdAt"`
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

	var totalBuyShares, totalSellShares decimal.Decimal
	var totalBuyCost, totalSellCost decimal.Decimal
	var totalBuyValue, totalSellValue decimal.Decimal

	for _, lot := range h.Lots {
		if lot.Type == "sell" {
			totalSellShares = totalSellShares.Add(lot.Shares)
			totalSellCost = totalSellCost.Add(lot.Cost)
			totalSellValue = totalSellValue.Add(lot.ValueAdded)
		} else {
			totalBuyShares = totalBuyShares.Add(lot.Shares)
			totalBuyCost = totalBuyCost.Add(lot.Cost)
			totalBuyValue = totalBuyValue.Add(lot.ValueAdded)
		}
	}

	if h.Symbol != "" {
		h.Shares = totalBuyShares.Sub(totalSellShares)
		h.Cost = totalBuyCost.Sub(totalSellCost)
		if h.Shares.Abs().LessThan(decimal.NewFromFloat(1e-9)) {
			h.Shares = decimal.Zero
		}
		if h.Shares.IsPositive() {
			h.CostPrice = h.Cost.Div(h.Shares)
		} else {
			h.CostPrice = decimal.Zero
		}
		h.Value = h.Shares.Mul(h.Price)
	} else {
		h.Shares = totalBuyShares.Sub(totalSellShares)
		h.Value = totalBuyValue.Sub(totalSellValue)
		h.Cost = totalBuyCost.Sub(totalSellCost)
		if h.Shares.Abs().LessThan(decimal.NewFromFloat(1e-9)) {
			h.Shares = decimal.Zero
		}
		if h.Shares.IsPositive() {
			h.CostPrice = h.Cost.Div(h.Shares)
		} else {
			h.CostPrice = decimal.Zero
		}
	}
}

// TotalFees returns the sum of all lot fees for this holding.
func (h *Holding) TotalFees() decimal.Decimal {
	total := decimal.Zero
	for _, lot := range h.Lots {
		total = total.Add(lot.Fee)
	}
	return total
}

// BuyFees returns the sum of buy lot fees only (excludes sell lot fees).
// Sell fees are already deducted from realizedValue, so including them
// in principal would double-count the cost.
func (h *Holding) BuyFees() decimal.Decimal {
	total := decimal.Zero
	for _, lot := range h.Lots {
		if lot.Type != "sell" {
			total = total.Add(lot.Fee)
		}
	}
	return total
}
