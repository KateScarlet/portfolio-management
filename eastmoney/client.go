package eastmoney

import (
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

var (
	client      *resty.Client
	quoteCache  sync.Map
	cacheExpiry = 5 * time.Minute

	symbolMarket = map[string]int{
		"au9999": 118,
		"agtd":   118,
		"scm":    142,
		"cum":    113,
	}

	symbolUnit = map[string]string{
		"au9999": "克",
		"agtd":   "千克",
		"scm":    "桶",
		"cum":    "吨",
	}
)

type cachedQuote struct {
	result    *PriceResult
	fetchedAt time.Time
}

type PriceResult struct {
	Symbol           string  `json:"symbol"`
	Name             string  `json:"name"`
	Price            float64 `json:"price"`
	OriginalPrice    float64 `json:"originalPrice"`
	Currency         string  `json:"currency"`
	OriginalCurrency string  `json:"originalCurrency"`
	Unit             string  `json:"unit"`
}

type eastmoneyResponse struct {
	RC   int `json:"rc"`
	Data *struct {
		F43 int    `json:"f43"`
		F57 string `json:"f57"`
		F58 string `json:"f58"`
		F59 int    `json:"f59"`
	} `json:"data"`
}

func Init() {
	client = resty.New().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetHeader("Accept", "application/json").
		SetTimeout(30 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			if err != nil {
				return true
			}
			status := r.StatusCode()
			return status == 429 || status >= 500
		})
}

func IsFuturesSymbol(symbol string) bool {
	_, ok := symbolMarket[strings.ToLower(symbol)]
	return ok
}

func getCachedQuote(symbol string) (*PriceResult, bool) {
	if v, ok := quoteCache.Load(symbol); ok {
		c := v.(*cachedQuote)
		if time.Since(c.fetchedAt) < cacheExpiry {
			return c.result, true
		}
		quoteCache.Delete(symbol)
	}
	return nil, false
}

func setCachedQuote(symbol string, result *PriceResult) {
	quoteCache.Store(symbol, &cachedQuote{result: result, fetchedAt: time.Now()})
}

func FetchQuote(symbol string) (*PriceResult, error) {
	if client == nil {
		return nil, fmt.Errorf("eastmoney client not initialized, call eastmoney.Init() first")
	}

	lower := strings.ToLower(symbol)
	market, ok := symbolMarket[lower]
	if !ok {
		return nil, fmt.Errorf("unknown eastmoney symbol: %s", symbol)
	}

	if cached, ok := getCachedQuote(lower); ok {
		slog.Info("eastmoney price fetched from cache", "symbol", symbol)
		return cached, nil
	}

	var resp eastmoneyResponse
	r, err := client.R().
		SetQueryParam("secid", fmt.Sprintf("%d.%s", market, lower)).
		SetQueryParam("fields", "f43,f57,f58,f59").
		SetResult(&resp).
		Get("https://push2.eastmoney.com/api/qt/stock/get")
	if err != nil {
		return nil, fmt.Errorf("eastmoney request failed: %w", err)
	}
	if r.IsError() {
		return nil, fmt.Errorf("eastmoney returned status %d", r.StatusCode())
	}

	if resp.RC != 0 || resp.Data == nil {
		return nil, fmt.Errorf("eastmoney no data for symbol %s", symbol)
	}

	price := float64(resp.Data.F43) / math.Pow(10, float64(resp.Data.F59))
	unit := symbolUnit[lower]

	result := &PriceResult{
		Symbol:           strings.ToUpper(resp.Data.F57),
		Name:             resp.Data.F58,
		Price:            price,
		OriginalPrice:    price,
		Currency:         "CNY",
		OriginalCurrency: "CNY",
		Unit:             unit,
	}

	slog.Info("eastmoney price fetched from API", "symbol", symbol, "price", price)
	setCachedQuote(lower, result)
	return result, nil
}
