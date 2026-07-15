package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type HoldingLot struct {
	ID         uuid.UUID       `gorm:"type:uuid;primaryKey;comment:批次唯一ID" json:"id"`
	HoldingID  uuid.UUID       `gorm:"type:uuid;index;not null;comment:关联持仓ID" json:"holdingId"`
	Type       string          `gorm:"size:10;default:'';comment:交易类型(buy/sell)" json:"type,omitempty"`
	Date       time.Time       `gorm:"comment:交易日期" json:"date"`
	Shares     decimal.Decimal `gorm:"type:decimal;default:0;comment:交易股数" json:"shares"`
	CostPrice  decimal.Decimal `gorm:"type:decimal;default:0;comment:成本单价" json:"costPrice"`
	Cost       decimal.Decimal `gorm:"type:decimal;default:0;comment:交易成本(买入为成本增加,卖出为成本减少)" json:"cost"`
	ValueAdded decimal.Decimal `gorm:"type:decimal;default:0;comment:市值变动(买入为购买时市值,卖出为卖出时市值)" json:"valueAdded"`
	Fee        decimal.Decimal `gorm:"type:decimal;default:0;comment:交易手续费" json:"fee"`
	Source     string          `gorm:"size:30;default:'trade';comment:批次来源(trade/dividend_reinvest)" json:"source,omitempty"`
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
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;comment:组合唯一ID" json:"id"`
	UserID      uuid.UUID `gorm:"type:uuid;index;not null;comment:所属用户ID" json:"userId"`
	Name        string    `gorm:"size:100;not null;comment:组合名称" json:"name"`
	Description string    `gorm:"size:500;default:'';comment:组合描述" json:"description,omitempty"`
	IsDefault   bool      `gorm:"default:false;comment:是否为默认组合" json:"isDefault"`
	CreatedAt   time.Time `gorm:"autoCreateTime;timestamptz;comment:创建时间" json:"createdAt"`
}

type Account struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;comment:账户唯一ID" json:"id"`
	UserID      uuid.UUID `gorm:"type:uuid;index;not null;comment:所属用户ID" json:"userId"`
	Name        string    `gorm:"size:100;not null;comment:账户名称" json:"name"`
	Description string    `gorm:"size:500;default:'';comment:账户描述" json:"description,omitempty"`
	Broker      string    `gorm:"size:100;default:'';comment:券商名称" json:"broker,omitempty"`
	IsDefault   bool      `gorm:"default:false;comment:是否为默认账户" json:"isDefault"`
	CreatedAt   time.Time `gorm:"autoCreateTime;timestamptz;comment:创建时间" json:"createdAt"`
}

type Holding struct {
	ID             uuid.UUID       `gorm:"type:uuid;primaryKey;comment:持仓唯一ID" json:"id"`
	UserID         uuid.UUID       `gorm:"type:uuid;index;not null;comment:所属用户ID" json:"userId"`
	PortfolioID    uuid.UUID       `gorm:"type:uuid;index;not null;comment:所属组合ID" json:"portfolioId"`
	AccountID      uuid.UUID       `gorm:"type:uuid;index;not null;comment:所属账户ID" json:"accountId"`
	AssetId        string          `gorm:"size:20;not null;comment:资产类型(stock/bond/fund等)" json:"assetId"`
	Symbol         string          `gorm:"size:20;default:'';comment:资产代码(如AAPL)" json:"symbol"`
	Name           string          `gorm:"size:200;default:'';comment:资产名称" json:"name,omitempty"`
	Market         string          `gorm:"size:20;default:'';comment:市场代码(如US/HK)" json:"market,omitempty"`
	Currency       string          `gorm:"size:10;default:'CNY';comment:计价货币" json:"currency"`
	Shares         decimal.Decimal `gorm:"type:decimal;default:0;comment:持有份额/股数" json:"shares"`
	Price          decimal.Decimal `gorm:"type:decimal;default:0;comment:当前价格" json:"price"`
	CostPrice      decimal.Decimal `gorm:"type:decimal;default:0;comment:成本单价" json:"costPrice"`
	Value          decimal.Decimal `gorm:"type:decimal;default:0;comment:当前市值(shares*price)" json:"value"`
	Cost           decimal.Decimal `gorm:"type:decimal;default:0;comment:净投入(买入支出-卖出净回款-现金分红)" json:"cost"`
	TotalDividends decimal.Decimal `gorm:"type:decimal;default:0;comment:累计已收分红" json:"totalDividends"`
	Date           time.Time       `gorm:"comment:建仓日期" json:"date"`
	Fee            decimal.Decimal `gorm:"-" json:"fee"`
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
	ID          uuid.UUID             `gorm:"type:uuid;primaryKey;comment:记录唯一ID" json:"id"`
	UserID      uuid.UUID             `gorm:"type:uuid;index;not null;comment:所属用户ID" json:"userId"`
	PortfolioID uuid.UUID             `gorm:"type:uuid;index;not null;comment:所属组合ID" json:"portfolioId"`
	Timestamp   time.Time             `gorm:"type:timestamptz;index;not null;comment:快照时间" json:"timestamp"`
	Assets      AssetMapColumn        `gorm:"type:text;not null;default:'{}';comment:各资产类型市值(JSON)" json:"assets"`
	Holdings    HoldingSnapshotColumn `gorm:"type:text;not null;default:'[]';comment:持仓快照(JSON数组)" json:"holdings"`
	Total       decimal.Decimal       `gorm:"type:decimal;default:0;comment:组合总资产" json:"total"`
	Principal   decimal.Decimal       `gorm:"type:decimal;default:0;comment:投入本金(买入成本+买入手续费)" json:"principal"`
}

type Setting struct {
	Key         string    `gorm:"primaryKey;size:100;comment:配置键" json:"key"`
	Value       string    `gorm:"not null;comment:配置值" json:"value"`
	UserID      uuid.UUID `gorm:"primaryKey;type:uuid;comment:所属用户ID" json:"userId,omitempty"`
	PortfolioID uuid.UUID `gorm:"primaryKey;type:uuid;comment:所属组合ID" json:"portfolioId,omitempty"`
}

type AvailableFund struct {
	ID          uuid.UUID       `gorm:"type:uuid;primaryKey;comment:记录唯一ID" json:"id"`
	UserID      uuid.UUID       `gorm:"type:uuid;index;uniqueIndex:idx_available_funds_unique;not null;comment:所属用户ID" json:"userId"`
	PortfolioID uuid.UUID       `gorm:"type:uuid;index;uniqueIndex:idx_available_funds_unique;not null;comment:所属组合ID" json:"portfolioId"`
	Currency    string          `gorm:"size:10;uniqueIndex:idx_available_funds_unique;not null;comment:货币代码(如CNY/USD)" json:"currency"`
	Amount      decimal.Decimal `gorm:"type:decimal;default:0;comment:可用金额" json:"amount"`
}

type FundTransaction struct {
	ID                uuid.UUID       `gorm:"type:uuid;primaryKey;comment:交易唯一ID" json:"id"`
	UserID            uuid.UUID       `gorm:"type:uuid;index;not null;comment:所属用户ID" json:"userId"`
	PortfolioID       uuid.UUID       `gorm:"type:uuid;index;not null;comment:所属组合ID" json:"portfolioId"`
	Type              string          `gorm:"size:20;not null;comment:交易类型(deposit/withdraw/transfer_in/transfer_out/dividend_cash/dividend_reinvest)" json:"type"`
	Amount            decimal.Decimal `gorm:"type:decimal;not null;comment:交易金额" json:"amount"`
	Currency          string          `gorm:"size:10;not null;comment:货币代码" json:"currency"`
	TargetPortfolioID *uuid.UUID      `gorm:"type:uuid;comment:划转目标组合ID(仅transfer类型)" json:"targetPortfolioId,omitempty"`
	TargetAmount      decimal.Decimal `gorm:"type:decimal;default:0;comment:划转目标金额(汇率换算后)" json:"targetAmount"`
	TargetCurrency    string          `gorm:"size:10;default:'';comment:划转目标货币" json:"targetCurrency,omitempty"`
	ExchangeRate      decimal.Decimal `gorm:"type:decimal;default:0;comment:汇率" json:"exchangeRate"`
	HoldingID         *uuid.UUID      `gorm:"type:uuid;comment:关联持仓ID(仅分红类型)" json:"holdingId,omitempty"`
	Note              string          `gorm:"size:500;default:'';comment:备注" json:"note,omitempty"`
	CreatedAt         time.Time       `gorm:"autoCreateTime;timestamptz;comment:创建时间" json:"createdAt"`
}

type User struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Username    string    `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Password    string    `gorm:"size:200;default:''" json:"-"`
	Role        string    `gorm:"size:20;default:'user'" json:"role"`
	SSOProvider string    `gorm:"size:20;default:''" json:"ssoProvider,omitempty"`
	SSOId       string    `gorm:"size:200;default:''" json:"-"`
	CreatedAt   time.Time `gorm:"autoCreateTime;timestamptz" json:"createdAt"`
}

// WebAuthnCredential stores a registered WebAuthn (passkey) credential for a user.
type WebAuthnCredential struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID       uuid.UUID `gorm:"type:uuid;index;not null" json:"userId"`
	Name         string    `gorm:"size:100;default:''" json:"name"`
	CredentialID []byte    `gorm:"type:bytes;not null" json:"-"`
	PublicKey    []byte    `gorm:"type:bytes;not null" json:"-"`
	Flags        uint8     `gorm:"default:0" json:"-"`
	SignCount    uint64    `gorm:"default:0" json:"-"`
	CreatedAt    time.Time `gorm:"autoCreateTime;timestamptz" json:"createdAt"`
	LastUsedAt   time.Time `gorm:"timestamptz" json:"lastUsedAt"`
}

type WebAuthnSession struct {
	ID        string    `gorm:"primaryKey"`
	Data      string    `gorm:"type:text;not null"`
	ExpiresAt time.Time `gorm:"type:timestamptz;not null"`
}

// RecalcFromLots recalculates holding fields from its lots.
// This is the single source of truth for financial calculations.
//
// Convention:
//   - Buy lot: Cost = raw cost (shares * costPrice, NO fee); ValueAdded = market value at purchase; Fee = transaction fee
//   - Sell lot: Cost = proportional cost reduction; ValueAdded = value removed from holding; Fee = transaction fee
//   - Holding: Cost is net investment: trade buys including fees, minus net
//     sale proceeds and cash dividends. Dividend reinvestment is an internal
//     transfer and therefore contributes zero net investment.
//   - Lot Cost remains the acquisition-cost basis used to calculate realized
//     gains when selling; it is deliberately separate from Holding.Cost.
func RecalcFromLots(h *Holding, lots []HoldingLot) {
	var totalBuyShares, totalSellShares decimal.Decimal
	var netInvestment, reinvestedDividends decimal.Decimal
	var totalBuyValue, totalSellValue decimal.Decimal

	for i := range lots {
		if lots[i].Type == "sell" {
			totalSellShares = totalSellShares.Add(lots[i].Shares)
			totalSellValue = totalSellValue.Add(lots[i].ValueAdded)
			netInvestment = netInvestment.Sub(lots[i].ValueAdded.Sub(lots[i].Fee))
		} else {
			totalBuyShares = totalBuyShares.Add(lots[i].Shares)
			totalBuyValue = totalBuyValue.Add(lots[i].ValueAdded)
			if lots[i].Source == "dividend_reinvest" {
				reinvestedDividends = reinvestedDividends.Add(lots[i].Cost)
			} else {
				netInvestment = netInvestment.Add(lots[i].Cost).Add(lots[i].Fee)
			}
		}
	}
	cashDividends := h.TotalDividends.Sub(reinvestedDividends)
	if cashDividends.IsPositive() {
		netInvestment = netInvestment.Sub(cashDividends)
	}

	if h.Symbol != "" {
		h.Shares = totalBuyShares.Sub(totalSellShares)
		h.Cost = netInvestment
		if h.Shares.IsPositive() {
			h.CostPrice = h.Cost.Div(h.Shares)
		} else {
			h.CostPrice = decimal.Zero
		}
		h.Value = h.Shares.Mul(h.Price)
	} else {
		h.Shares = totalBuyShares.Sub(totalSellShares)
		h.Value = totalBuyValue.Sub(totalSellValue)
		h.Cost = netInvestment
		if h.Shares.IsPositive() {
			h.CostPrice = h.Cost.Div(h.Shares)
		} else {
			h.CostPrice = decimal.Zero
		}
	}
}

// CostBasis returns the remaining acquisition-cost basis represented by lots.
// Unlike Holding.Cost (net investment), it includes dividend-reinvestment lots
// and is reduced by the basis allocated to prior sales.
func CostBasis(lots []HoldingLot) decimal.Decimal {
	total := decimal.Zero
	for i := range lots {
		if lots[i].Type == "sell" {
			total = total.Sub(lots[i].Cost)
		} else {
			total = total.Add(lots[i].Cost)
		}
	}
	return total
}

// TotalFees returns the sum of all lot fees for this holding.
func TotalFees(lots []HoldingLot) decimal.Decimal {
	total := decimal.Zero
	for i := range lots {
		total = total.Add(lots[i].Fee)
	}
	return total
}

// BuyFees returns the sum of buy lot fees only (excludes sell lot fees).
// Sell fees are already deducted from realizedValue, so including them
// in principal would double-count the cost.
func BuyFees(lots []HoldingLot) decimal.Decimal {
	total := decimal.Zero
	for i := range lots {
		if lots[i].Type != "sell" {
			total = total.Add(lots[i].Fee)
		}
	}
	return total
}

// SellFees returns the sum of sell lot fees only (excludes buy lot fees).
func SellFees(lots []HoldingLot) decimal.Decimal {
	total := decimal.Zero
	for i := range lots {
		if lots[i].Type == "sell" {
			total = total.Add(lots[i].Fee)
		}
	}
	return total
}

type Dividend struct {
	ID                uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	UserID            uuid.UUID       `gorm:"type:uuid;index;not null" json:"userId"`
	PortfolioID       uuid.UUID       `gorm:"type:uuid;index;not null" json:"portfolioId"`
	HoldingID         uuid.UUID       `gorm:"type:uuid;index;not null" json:"holdingId"`
	Type              string          `gorm:"size:16;not null" json:"type"`
	GrossAmount       decimal.Decimal `gorm:"type:decimal;not null" json:"grossAmount"`
	TaxAmount         decimal.Decimal `gorm:"type:decimal;not null;default:0" json:"taxAmount"`
	NetAmount         decimal.Decimal `gorm:"type:decimal;not null" json:"netAmount"`
	Currency          string          `gorm:"size:10;not null" json:"currency"`
	PaymentDate       time.Time       `gorm:"timestamptz;not null;index" json:"paymentDate"`
	SharesAtPayment   decimal.Decimal `gorm:"type:decimal;not null;default:0" json:"sharesAtPayment"`
	ReinvestmentPrice decimal.Decimal `gorm:"type:decimal;not null;default:0" json:"reinvestmentPrice"`
	ReinvestedShares  decimal.Decimal `gorm:"type:decimal;not null;default:0" json:"reinvestedShares"`
	HoldingLotID      *uuid.UUID      `gorm:"type:uuid" json:"holdingLotId,omitempty"`
	FundTxID          uuid.UUID       `gorm:"type:uuid;not null" json:"fundTxId"`
	Note              string          `gorm:"size:500;not null;default:''" json:"note,omitempty"`
	CreatedAt         time.Time       `gorm:"autoCreateTime;timestamptz" json:"createdAt"`
	UpdatedAt         time.Time       `gorm:"autoUpdateTime;timestamptz" json:"updatedAt"`
}

// TableName deliberately uses a new table. The old dividends table represented
// a different accounting contract and is not migrated.
func (Dividend) TableName() string { return "dividend_events" }
