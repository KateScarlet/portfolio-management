package eastmoney

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
)

var (
	client      *resty.Client
	quoteCache  sync.Map
	cacheExpiry = 5 * time.Minute

	fundCodeRe = regexp.MustCompile(`^\d{6}$`)

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

func ClearCache() {
	quoteCache = sync.Map{}
}

func IsFundCode(code string) bool {
	return fundCodeRe.MatchString(code)
}

func convertAShareSymbol(code string) string {
	s := strings.ToUpper(code)
	if s[0] == '6' || s[0] == '5' || s[0] == '7' || s[0] == '9' {
		return "1." + s
	}
	return "0." + s
}

func FetchAShareQuote(symbol string) (*PriceResult, error) {
	if client == nil {
		return nil, fmt.Errorf("eastmoney client not initialized, call eastmoney.Init() first")
	}

	secid := convertAShareSymbol(symbol)
	cacheKey := "cn:" + secid
	if cached, ok := getCachedQuote(cacheKey); ok {
		slog.Info("eastmoney A-share price fetched from cache", "symbol", symbol)
		return cached, nil
	}

	var resp eastmoneyResponse
	r, err := client.R().
		SetQueryParam("secid", secid).
		SetQueryParam("fields", "f43,f57,f58,f59").
		SetResult(&resp).
		Get("https://push2.eastmoney.com/api/qt/stock/get")
	if err != nil {
		return nil, fmt.Errorf("eastmoney A-share request failed: %w", err)
	}
	if r.IsError() {
		return nil, fmt.Errorf("eastmoney A-share returned status %d", r.StatusCode())
	}

	if resp.RC != 0 || resp.Data == nil {
		return nil, fmt.Errorf("eastmoney no data for A-share %s", symbol)
	}

	price := float64(resp.Data.F43) / math.Pow(10, float64(resp.Data.F59))

	result := &PriceResult{
		Symbol:           strings.ToUpper(resp.Data.F57),
		Name:             resp.Data.F58,
		Price:            price,
		OriginalPrice:    price,
		Currency:         "CNY",
		OriginalCurrency: "CNY",
	}

	slog.Info("eastmoney A-share price fetched from API", "symbol", symbol, "price", price)
	setCachedQuote(cacheKey, result)
	return result, nil
}

type fundGZResponse struct {
	FundCode string `json:"fundcode"`
	Name     string `json:"name"`
	JZRQ     string `json:"jzrq"`
	DWJZ     string `json:"dwjz"`
	GSZ      string `json:"gsz"`
	GSZSZL   string `json:"gszzl"`
	GZTime   string `json:"gztime"`
}

var jsonpRe = regexp.MustCompile(`^jsonpgz\((.*)\);$`)

func FetchFundQuote(code string) (*PriceResult, error) {
	if client == nil {
		return nil, fmt.Errorf("eastmoney client not initialized, call eastmoney.Init() first")
	}

	cacheKey := "fund:" + code
	if cached, ok := getCachedQuote(cacheKey); ok {
		slog.Info("eastmoney fund price fetched from cache", "code", code)
		return cached, nil
	}

	r, err := client.R().
		Get(fmt.Sprintf("https://fundgz.1234567.com.cn/js/%s.js", code))
	if err != nil {
		return nil, fmt.Errorf("eastmoney fund request failed: %w", err)
	}
	if r.IsError() {
		return nil, fmt.Errorf("eastmoney fund returned status %d", r.StatusCode())
	}

	body := strings.TrimSpace(r.String())
	m := jsonpRe.FindStringSubmatch(body)
	if len(m) < 2 {
		return nil, fmt.Errorf("unexpected fund response format for %s: %s", code, body[:min(len(body), 200)])
	}

	var resp fundGZResponse
	if err := json.Unmarshal([]byte(m[1]), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse fund response for %s: %w", code, err)
	}

	var price float64
	if resp.GSZ != "" {
		fmt.Sscanf(resp.GSZ, "%f", &price)
	} else if resp.DWJZ != "" {
		fmt.Sscanf(resp.DWJZ, "%f", &price)
	}
	if price == 0 {
		return nil, fmt.Errorf("no price for fund %s", code)
	}

	result := &PriceResult{
		Symbol:           resp.FundCode,
		Name:             resp.Name,
		Price:            price,
		OriginalPrice:    price,
		Currency:         "CNY",
		OriginalCurrency: "CNY",
	}

	slog.Info("eastmoney fund price fetched from API", "code", code, "price", price)
	setCachedQuote(cacheKey, result)
	return result, nil
}
