package notifications

import (
	"portfolio-management/models"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// Channel defines the interface for notification channels.
type Channel interface {
	// Name returns the channel name for logging.
	Name() string
	// IsEnabled checks if this channel is enabled for the given portfolio.
	IsEnabled(userID, portfolioID uuid.UUID, db *gorm.DB) bool
	// ShouldSendSummary checks if a summary should be sent based on the channel's interval setting.
	ShouldSendSummary(userID, portfolioID uuid.UUID, lastTime, now time.Time, db *gorm.DB) bool
	// SendPriceAlert sends a price volatility alert.
	SendPriceAlert(userID, portfolioID uuid.UUID, portfolioName string, alerts []PriceAlert, db *gorm.DB) error
	// SendDriftAlert sends an allocation drift alert.
	SendDriftAlert(userID, portfolioID uuid.UUID, portfolioName string, alerts []DriftAlert, db *gorm.DB) error
	// SendSummary sends a portfolio summary.
	SendSummary(userID, portfolioID uuid.UUID, portfolioName string, data SummaryData, db *gorm.DB) error
}

// PriceAlert represents a single price change alert item.
type PriceAlert struct {
	Arrow     string // 📈 / 📉
	Name      string
	Symbol    string
	Price     string
	ChangePct string
}

// DriftAlert represents a single asset allocation drift item.
type DriftAlert struct {
	AssetName string
	Pct       string
	Target    string
	Diff      string
}

// SummaryData contains the data needed to render a portfolio summary.
type SummaryData struct {
	Total     string
	Principal string
	PnL       string // empty if not applicable
	Date      string
	Assets    []SummaryAsset
}

// SummaryAsset is one asset class in the summary.
type SummaryAsset struct {
	Name  string
	Pct   string
	Value string
}

// StateKey creates a composite key from userID and portfolioID for per-portfolio state.
func StateKey(userID, portfolioID uuid.UUID) string {
	return userID.String() + ":" + portfolioID.String()
}

// LoadPortfolioSettings loads all settings for a portfolio.
func LoadPortfolioSettings(db *gorm.DB, portfolioID uuid.UUID) map[string]string {
	var settings []models.Setting
	if err := db.Where("portfolio_id = ?", portfolioID).Find(&settings).Error; err != nil {
		return nil
	}
	result := make(map[string]string, len(settings))
	for i := range settings {
		result[settings[i].Key] = settings[i].Value
	}
	return result
}

// LoadSetting loads a single setting value for a user+portfolio.
func LoadSetting(db *gorm.DB, userID, portfolioID uuid.UUID, key string) string {
	var s models.Setting
	if err := db.Where("user_id = ? AND portfolio_id = ? AND key = ?", userID, portfolioID, key).First(&s).Error; err != nil {
		return ""
	}
	return s.Value
}

// ParseThreshold parses a threshold from settings, trying keys in order.
func ParseThreshold(settings map[string]string, keys ...string) decimal.Decimal {
	threshold := decimal.NewFromInt(5)
	for _, k := range keys {
		if v := settings[k]; v != "" {
			if t, err := decimal.NewFromString(v); err == nil {
				threshold = t
				break
			}
		}
	}
	return threshold
}
