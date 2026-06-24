package yahoo

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

var (
	client   *resty.Client
	aShareRe = regexp.MustCompile(`^\d{6}$`)
	shTagRe  = regexp.MustCompile(`^SH\d{6}$`)
	szTagRe  = regexp.MustCompile(`^SZ\d{6}$`)
	hkTagRe  = regexp.MustCompile(`^HK\d{4,5}$`)
	hkCodeRe = regexp.MustCompile(`^\d{4,5}$`)

	rateCache   sync.Map
	quoteCache  sync.Map
	cacheExpiry = 5 * time.Minute
)

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

type cachedRate struct {
	rate      float64
	fetchedAt time.Time
}

type cachedQuote struct {
	result    *PriceResult
	fetchedAt time.Time
}

func getCachedRate(pair string) (float64, bool) {
	if v, ok := rateCache.Load(pair); ok {
		c := v.(*cachedRate)
		if time.Since(c.fetchedAt) < cacheExpiry {
			return c.rate, true
		}
		rateCache.Delete(pair)
	}
	return 0, false
}

func setCachedRate(pair string, rate float64) {
	rateCache.Store(pair, &cachedRate{rate: rate, fetchedAt: time.Now()})
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

func ClearRateCache() {
	rateCache = sync.Map{}
	quoteCache = sync.Map{}
}

type YahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Symbol             string  `json:"symbol"`
				Currency           string  `json:"currency"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				ShortName          string  `json:"shortName"`
				LongName           string  `json:"longName"`
			} `json:"meta"`
		} `json:"result"`
	} `json:"chart"`
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

func ConvertSymbol(symbol string) string {
	s := strings.ToUpper(symbol)
	if aShareRe.MatchString(s) {
		// Shanghai: 5xxxxx (funds/ETFs), 6xxxxx (stocks)
		if s[0] == '5' || s[0] == '6' {
			return s + ".SS"
		}
		// Shenzhen: 0xxxxx (stocks), 2xxxxx (B-shares), 3xxxxx (ChiNext)
		if s[0] == '0' || s[0] == '2' || s[0] == '3' {
			return s + ".SZ"
		}
		// Shenzhen ETFs: 159xxx
		if strings.HasPrefix(s, "159") {
			return s + ".SZ"
		}
		// Shenzhen convertible bonds: 127xxx, 128xxx
		if strings.HasPrefix(s, "127") || strings.HasPrefix(s, "128") {
			return s + ".SZ"
		}
		// All other 6-digit codes (1xxxxx, 4xxxxx, 7xxxxx, 8xxxxx, 9xxxxx)
		// default to Shanghai, which has more bond/convertible listings
		return s + ".SS"
	}
	if shTagRe.MatchString(s) {
		return s[2:] + ".SS"
	}
	if szTagRe.MatchString(s) {
		return s[2:] + ".SZ"
	}
	if hkTagRe.MatchString(s) {
		return s[2:] + ".HK"
	}
	// HK stocks: 4-5 digit codes without prefix
	if hkCodeRe.MatchString(s) {
		return s + ".HK"
	}
	return s
}

func FetchQuote(symbol string) (*PriceResult, error) {
	if client == nil {
		return nil, fmt.Errorf("yahoo client not initialized, call yahoo.Init() first")
	}
	querySymbol := ConvertSymbol(symbol)

	if cached, ok := getCachedQuote(querySymbol); ok {
		slog.Info("price fetched from cache", "symbol", symbol, "querySymbol", querySymbol)
		return cached, nil
	}

	var result YahooChartResponse
	resp, err := client.R().
		SetQueryParam("range", "1d").
		SetQueryParam("interval", "1d").
		SetResult(&result).
		Get(fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s", querySymbol))
	if err != nil {
		return nil, fmt.Errorf("yahoo finance request failed: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("yahoo finance returned status %d", resp.StatusCode())
	}

	if len(result.Chart.Result) == 0 {
		return nil, fmt.Errorf("no result for symbol %s", symbol)
	}

	meta := result.Chart.Result[0].Meta
	if meta.RegularMarketPrice == 0 {
		return nil, fmt.Errorf("no price for symbol %s", symbol)
	}

	name := meta.ShortName
	if name == "" {
		name = meta.LongName
	}
	if name == "" {
		name = meta.Symbol
	}

	currency := meta.Currency
	price := meta.RegularMarketPrice
	originalPrice := meta.RegularMarketPrice

	priceResult := &PriceResult{
		Symbol:           meta.Symbol,
		Name:             name,
		Price:            price,
		OriginalPrice:    originalPrice,
		Currency:         currency,
		OriginalCurrency: currency,
	}
	slog.Info("price fetched from API", "symbol", symbol, "querySymbol", querySymbol)
	setCachedQuote(querySymbol, priceResult)
	return priceResult, nil
}

func FetchExchangeRate(pair string) (float64, error) {
	if rate, ok := getCachedRate(pair); ok {
		slog.Info("exchange rate fetched from cache", "pair", pair)
		return rate, nil
	}

	if client == nil {
		return 0, fmt.Errorf("yahoo client not initialized, call yahoo.Init() first")
	}
	fxSymbol := pair + "=X"
	var result YahooChartResponse
	resp, err := client.R().
		SetQueryParam("range", "1d").
		SetQueryParam("interval", "1d").
		SetResult(&result).
		Get(fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s", fxSymbol))
	if err != nil {
		return 0, fmt.Errorf("exchange rate request failed: %w", err)
	}
	if resp.IsError() {
		return 0, fmt.Errorf("exchange rate returned status %d", resp.StatusCode())
	}

	if len(result.Chart.Result) == 0 {
		return 0, fmt.Errorf("no exchange rate for %s", pair)
	}

	rate := result.Chart.Result[0].Meta.RegularMarketPrice
	if rate == 0 {
		return 0, fmt.Errorf("zero exchange rate for %s", pair)
	}

	slog.Info("exchange rate fetched from API", "pair", pair)
	setCachedRate(pair, rate)
	return rate, nil
}
