package marketsource

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"portfolio-management/models"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"gorm.io/gorm"
)

type userConfigEntry struct {
	config map[string][]string
}

type Router struct {
	db         *gorm.DB
	sources    map[string]MarketSource
	defaults   map[string][]string
	quoteCache *ttlcache.Cache[string, *Quote]
	rateCache  *ttlcache.Cache[string, float64]
	userCache  *ttlcache.Cache[string, *userConfigEntry]
	cacheTTL   time.Duration
}

func NewRouter(db *gorm.DB, sources map[string]MarketSource) *Router {
	r := &Router{
		db:      db,
		sources: sources,
		defaults: map[string][]string{
			"US":             {"eastmoney", "sina", "yahoo"},
			"HK":             {"eastmoney", "tencent", "sina", "yahoo"},
			"CRYPTO":         {"coingecko", "yahoo"},
			"COMMODITY_INTL": {"yahoo"},
			"CN":             {"eastmoney", "tencent", "sina", "yahoo"},
			"FUND":           {"eastmoney"},
			"COMMODITY_CN":   {"eastmoney"},
		},
		cacheTTL: 5 * time.Minute,
	}
	r.quoteCache = ttlcache.New[string, *Quote]()
	r.rateCache = ttlcache.New[string, float64]()
	r.userCache = ttlcache.New[string, *userConfigEntry]()
	go r.quoteCache.Start()
	go r.rateCache.Start()
	go r.userCache.Start()
	return r
}

func (r *Router) FetchQuote(userID, symbol, market string) (*Quote, error) {
	cacheKey := symbol + ":" + market
	if item := r.quoteCache.Get(cacheKey); item != nil {
		q := item.Value()
		slog.Info("price fetched from cache", "source", "cache", "symbol", symbol, "market", market)
		return q, nil
	}

	sources := r.resolveSources(userID, market)

	slog.Info("fetching quote", "userId", userID, "symbol", symbol, "market", market, "sources", sources)

	var lastErr error
	for _, name := range sources {
		src, ok := r.sources[name]
		if !ok {
			continue
		}
		q, err := src.FetchQuote(symbol, market)
		if err != nil {
			slog.Warn("source fetch failed, trying next",
				"source", name, "symbol", symbol, "market", market, "error", err)
			lastErr = err
			continue
		}
		slog.Info("price fetched", "source", name, "symbol", symbol, "market", market)
		r.quoteCache.Set(cacheKey, q, r.cacheTTL)
		return q, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no source available for market %s", market)
}

func (r *Router) ExchangeRate(userID, pair string) (float64, error) {
	if item := r.rateCache.Get(pair); item != nil {
		slog.Info("exchange rate fetched from cache", "source", "cache", "pair", pair)
		return item.Value(), nil
	}

	sources := r.resolveSources(userID, "")

	var lastErr error
	for _, name := range sources {
		src, ok := r.sources[name]
		if !ok {
			continue
		}
		rate, err := src.FetchExchangeRate(pair)
		if err != nil {
			if errors.Is(err, ErrNotSupported) {
				continue
			}
			slog.Warn("exchange rate fetch failed, trying next",
				"source", name, "pair", pair, "error", err)
			lastErr = err
			continue
		}
		slog.Info("exchange rate fetched", "source", name, "pair", pair)
		r.rateCache.Set(pair, rate, r.cacheTTL)
		return rate, nil
	}

	if lastErr != nil {
		return 0, lastErr
	}
	return 0, fmt.Errorf("no source available for exchange rate %s", pair)
}

func (r *Router) ClearAllCaches() {
	r.quoteCache.DeleteAll()
	r.rateCache.DeleteAll()
	r.userCache.DeleteAll()
}

func (r *Router) SourceNames() map[string]string {
	result := make(map[string]string, len(r.sources))
	for name, src := range r.sources {
		result[name] = src.Name()
	}
	return result
}

func (r *Router) AvailableSources() map[string][]string {
	result := make(map[string][]string)
	for name, src := range r.sources {
		for _, m := range src.SupportedMarkets() {
			result[m] = append(result[m], name)
		}
	}
	return result
}

func (r *Router) GetUserConfig(userID string) map[string][]string {
	if userID != "" {
		if cfg := r.loadUserConfig(userID); cfg != nil {
			result := make(map[string][]string, len(cfg))
			for k, v := range cfg {
				cp := make([]string, len(v))
				copy(cp, v)
				result[k] = cp
			}
			return result
		}
	}
	result := make(map[string][]string, len(r.defaults))
	for k, v := range r.defaults {
		cp := make([]string, len(v))
		copy(cp, v)
		result[k] = cp
	}
	return result
}

func (r *Router) UpdateUserConfig(userID string, config map[string][]string) error {
	if err := r.validateConfig(config); err != nil {
		return err
	}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	setting := models.Setting{
		Key:         "marketSources",
		Value:       string(data),
		UserID:      userID,
		PortfolioID: "",
	}

	// Try to find existing record first
	var existing models.Setting
	result := r.db.Where("key = ? AND user_id = ? AND portfolio_id = ?", "marketSources", userID, "").First(&existing)
	if result.Error == nil {
		// Update existing record
		result = r.db.Model(&existing).Update("value", string(data))
	} else {
		// Create new record
		result = r.db.Create(&setting)
	}
	if result.Error != nil {
		return result.Error
	}
	slog.Info("market source config saved", "userId", userID, "config", string(data))
	r.userCache.Delete(userID)
	r.quoteCache.DeleteAll()
	return nil
}

func (r *Router) resolveSources(userID, market string) []string {
	if market == "" {
		slog.Debug("resolveSources: no market, using all sources")
		return r.allSourceNames()
	}
	if userID != "" {
		if cfg := r.loadUserConfig(userID); cfg != nil {
			if sources, ok := cfg[market]; ok && len(sources) > 0 {
				slog.Info("resolveSources: using user config", "userId", userID, "market", market, "sources", sources)
				return sources
			}
		}
	}
	if sources, ok := r.defaults[market]; ok {
		slog.Info("resolveSources: using defaults", "market", market, "sources", sources)
		return sources
	}
	slog.Info("resolveSources: using all sources", "market", market)
	return r.allSourceNames()
}

func (r *Router) allSourceNames() []string {
	names := make([]string, 0, len(r.sources))
	for name := range r.sources {
		names = append(names, name)
	}
	return names
}

func (r *Router) loadUserConfig(userID string) map[string][]string {
	if item := r.userCache.Get(userID); item != nil {
		return item.Value().config
	}

	var value string
	err := r.db.Table("settings").
		Where("`key` = ? AND user_id = ? AND portfolio_id = ?", "marketSources", userID, "").
		Pluck("value", &value).Error
	if err != nil {
		slog.Error("failed to load user config from db", "userId", userID, "error", err)
		r.userCache.Set(userID, &userConfigEntry{config: nil}, time.Minute)
		return nil
	}
	if value == "" {
		slog.Debug("no user config found in db", "userId", userID)
		r.userCache.Set(userID, &userConfigEntry{config: nil}, time.Minute)
		return nil
	}

	var cfg map[string][]string
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		slog.Error("failed to unmarshal user config", "userId", userID, "error", err)
		r.userCache.Set(userID, &userConfigEntry{config: nil}, time.Minute)
		return nil
	}
	slog.Debug("loaded user config", "userId", userID, "config", cfg)
	r.userCache.Set(userID, &userConfigEntry{config: cfg}, time.Minute)
	return cfg
}

func (r *Router) validateConfig(config map[string][]string) error {
	available := r.AvailableSources()
	for market, sources := range config {
		validSources, ok := available[market]
		if !ok {
			return fmt.Errorf("unknown market: %s", market)
		}
		validSet := make(map[string]bool, len(validSources))
		for _, s := range validSources {
			validSet[s] = true
		}
		for _, s := range sources {
			if !validSet[s] {
				return fmt.Errorf("source %s not available for market %s", s, market)
			}
		}
	}
	return nil
}
