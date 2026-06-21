package yahoo

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

var (
	client     *resty.Client
	aShareRe   = regexp.MustCompile(`^\d{6}$`)
	shPrefixRe = regexp.MustCompile(`^[56]\d{5}$`)
	szPrefixRe = regexp.MustCompile(`^[0123]\d{5}$`)
	shTagRe    = regexp.MustCompile(`^SH\d{6}$`)
	szTagRe    = regexp.MustCompile(`^SZ\d{6}$`)

	rateCache   sync.Map
	cacheExpiry = 5 * time.Minute
)

func Init() {
	client = resty.New().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetHeader("Accept", "application/json").
		SetTimeout(30 * time.Second)
}

type cachedRate struct {
	rate      float64
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

func ClearRateCache() {
	rateCache = sync.Map{}
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
		if shPrefixRe.MatchString(s) {
			return s + ".SS"
		}
		if szPrefixRe.MatchString(s) {
			return s + ".SZ"
		}
	}
	if shTagRe.MatchString(s) {
		return s[2:] + ".SS"
	}
	if szTagRe.MatchString(s) {
		return s[2:] + ".SZ"
	}
	return s
}

func FetchQuote(symbol string) (*PriceResult, error) {
	if client == nil {
		return nil, fmt.Errorf("yahoo client not initialized, call yahoo.Init() first")
	}
	querySymbol := ConvertSymbol(symbol)

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
	unit := ""
	upperSymbol := strings.ToUpper(meta.Symbol)
	isPreciousMetal := upperSymbol == "GC=F" || upperSymbol == "SI=F" || upperSymbol == "PL=F" || upperSymbol == "PA=F"

	if currency != "" && currency != "CNY" {
		pair := fmt.Sprintf("%sCNY", currency)
		rate, err := FetchExchangeRate(pair)
		if err != nil {
			return nil, fmt.Errorf("fx conversion failed for %s->CNY: %w", currency, err)
		}
		price *= rate
	}

	if isPreciousMetal {
		price /= 31.1035
		unit = "克"
	}

	return &PriceResult{
		Symbol:           meta.Symbol,
		Name:             name,
		Price:            price,
		OriginalPrice:    originalPrice,
		Currency:         "CNY",
		OriginalCurrency: currency,
		Unit:             unit,
	}, nil
}

func FetchExchangeRate(pair string) (float64, error) {
	if rate, ok := getCachedRate(pair); ok {
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

	setCachedRate(pair, rate)
	return rate, nil
}
