package marketsource

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"portfolio-management/models"
	"sync"
	"time"

	"gorm.io/gorm"
)

type cachedEntry struct {
	data      any
	source    string
	fetchedAt time.Time
}

type userConfigEntry struct {
	config    map[string][]string
	fetchedAt time.Time
}

type Router struct {
	db         *gorm.DB
	sources    map[string]MarketSource
	defaults   map[string][]string
	quoteCache sync.Map
	rateCache  sync.Map
	userCache  sync.Map
	cacheTTL   time.Duration
}

func NewRouter(db *gorm.DB, sources map[string]MarketSource) *Router {
	return &Router{
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
}

func (r *Router) FetchQuote(userID, symbol, market string) (*Quote, error) {
	cacheKey := symbol + ":" + market
	if v, ok := r.quoteCache.Load(cacheKey); ok {
		e := v.(*cachedEntry)
		if time.Since(e.fetchedAt) < r.cacheTTL {
			slog.Info("price fetched from cache", "source", e.source, "symbol", symbol, "market", market)
			return e.data.(*Quote), nil
		}
		r.quoteCache.Delete(cacheKey)
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
		r.quoteCache.Store(cacheKey, &cachedEntry{data: q, source: name, fetchedAt: time.Now()})
		return q, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no source available for market %s", market)
}

func (r *Router) ExchangeRate(userID, pair string) (float64, error) {
	if v, ok := r.rateCache.Load(pair); ok {
		e := v.(*cachedEntry)
		if time.Since(e.fetchedAt) < r.cacheTTL {
			slog.Info("exchange rate fetched from cache", "source", e.source, "pair", pair)
			return e.data.(float64), nil
		}
		r.rateCache.Delete(pair)
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
		r.rateCache.Store(pair, &cachedEntry{data: rate, source: name, fetchedAt: time.Now()})
		return rate, nil
	}

	if lastErr != nil {
		return 0, lastErr
	}
	return 0, fmt.Errorf("no source available for exchange rate %s", pair)
}

func (r *Router) ClearAllCaches() {
	r.quoteCache = sync.Map{}
	r.rateCache = sync.Map{}
	r.userCache = sync.Map{}
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
	// Clear quote cache so next sync uses new source order
	r.quoteCache = sync.Map{}
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
	if v, ok := r.userCache.Load(userID); ok {
		e := v.(*userConfigEntry)
		if time.Since(e.fetchedAt) < time.Minute {
			return e.config
		}
		r.userCache.Delete(userID)
	}

	var value string
	err := r.db.Table("settings").
		Where("`key` = ? AND user_id = ? AND portfolio_id = ?", "marketSources", userID, "").
		Pluck("value", &value).Error
	if err != nil {
		slog.Error("failed to load user config from db", "userId", userID, "error", err)
		r.userCache.Store(userID, &userConfigEntry{config: nil, fetchedAt: time.Now()})
		return nil
	}
	if value == "" {
		slog.Debug("no user config found in db", "userId", userID)
		r.userCache.Store(userID, &userConfigEntry{config: nil, fetchedAt: time.Now()})
		return nil
	}

	var cfg map[string][]string
	if err := json.Unmarshal([]byte(value), &cfg); err != nil {
		slog.Error("failed to unmarshal user config", "userId", userID, "error", err)
		r.userCache.Store(userID, &userConfigEntry{config: nil, fetchedAt: time.Now()})
		return nil
	}
	slog.Debug("loaded user config", "userId", userID, "config", cfg)
	r.userCache.Store(userID, &userConfigEntry{config: cfg, fetchedAt: time.Now()})
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
