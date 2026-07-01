package eastmoney

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

	"portfolio-management/marketsource"
)

var (
	httpClient *resty.Client

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
	httpClient = resty.New().
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

type Client struct{}

func (c *Client) Name() string { return "东方财富" }

func (c *Client) SupportedMarkets() []string {
	return []string{"CN", "FUND", "COMMODITY_CN", "US", "HK"}
}

func (c *Client) FetchQuote(symbol, market string) (*marketsource.Quote, error) {
	switch market {
	case "FUND":
		return fetchFundQuote(symbol)
	case "CN":
		return fetchAShareQuote(symbol)
	case "US":
		return fetchUSStockQuote(symbol)
	case "HK":
		return fetchHKStockQuote(symbol)
	default:
		return fetchCommodityQuote(symbol)
	}
}

func (c *Client) FetchExchangeRate(pair string) (float64, error) {
	return 0, marketsource.ErrNotSupported
}

func IsFuturesSymbol(symbol string) bool {
	_, ok := symbolMarket[strings.ToLower(symbol)]
	return ok
}

func fetchCommodityQuote(symbol string) (*marketsource.Quote, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("eastmoney client not initialized, call eastmoney.Init() first")
	}

	// symbol is canonical like "au9999.CN", normalize for source
	querySymbol := marketsource.NormalizeForSource(symbol, "COMMODITY_CN", "eastmoney")
	lower := strings.ToLower(querySymbol)
	market, ok := symbolMarket[lower]
	if !ok {
		return nil, fmt.Errorf("unknown eastmoney symbol: %s", symbol)
	}

	var resp eastmoneyResponse
	r, err := httpClient.R().
		SetQueryParam("secid", fmt.Sprintf("%d.%s", market, lower)).
		SetQueryParam("fields", "f43,f57,f58,f59").
		SetResult(&resp).
		Get("http://push2.eastmoney.com/api/qt/stock/get")
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

	slog.Info("eastmoney price fetched from API", "symbol", symbol, "price", price)
	return &marketsource.Quote{
		Symbol:           symbol,
		Name:             resp.Data.F58,
		Price:            price,
		OriginalPrice:    price,
		Currency:         "CNY",
		OriginalCurrency: "CNY",
		Unit:             unit,
	}, nil
}

func fetchAShareQuote(symbol string) (*marketsource.Quote, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("eastmoney client not initialized, call eastmoney.Init() first")
	}

	// symbol is canonical like "600519.SH", normalize for source
	secid := marketsource.NormalizeForSource(symbol, "CN", "eastmoney")

	var resp eastmoneyResponse
	r, err := httpClient.R().
		SetQueryParam("secid", secid).
		SetQueryParam("fields", "f43,f57,f58,f59").
		SetResult(&resp).
		Get("http://push2.eastmoney.com/api/qt/stock/get")
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

	slog.Info("eastmoney A-share price fetched from API", "symbol", symbol, "price", price)
	return &marketsource.Quote{
		Symbol:           symbol,
		Name:             resp.Data.F58,
		Price:            price,
		OriginalPrice:    price,
		Currency:         "CNY",
		OriginalCurrency: "CNY",
	}, nil
}

func fetchUSStockQuote(symbol string) (*marketsource.Quote, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("eastmoney client not initialized, call eastmoney.Init() first")
	}

	// symbol is canonical like "AAPL.US", normalize for source
	secid := marketsource.NormalizeForSource(symbol, "US", "eastmoney")

	var resp eastmoneyResponse
	r, err := httpClient.R().
		SetQueryParam("secid", secid).
		SetQueryParam("fields", "f43,f57,f58,f59").
		SetResult(&resp).
		Get("http://push2.eastmoney.com/api/qt/stock/get")
	if err != nil {
		return nil, fmt.Errorf("eastmoney US stock request failed: %w", err)
	}
	if r.IsError() {
		return nil, fmt.Errorf("eastmoney US stock returned status %d", r.StatusCode())
	}

	if resp.RC != 0 || resp.Data == nil {
		return nil, fmt.Errorf("eastmoney no data for US stock %s", symbol)
	}

	price := float64(resp.Data.F43) / math.Pow(10, float64(resp.Data.F59))

	slog.Info("eastmoney US stock price fetched from API", "symbol", symbol, "price", price)
	return &marketsource.Quote{
		Symbol:           symbol,
		Name:             resp.Data.F58,
		Price:            price,
		OriginalPrice:    price,
		Currency:         "USD",
		OriginalCurrency: "USD",
	}, nil
}

func fetchHKStockQuote(symbol string) (*marketsource.Quote, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("eastmoney client not initialized, call eastmoney.Init() first")
	}

	// symbol is canonical like "00700.HK", normalize for source
	secid := marketsource.NormalizeForSource(symbol, "HK", "eastmoney")

	var resp eastmoneyResponse
	r, err := httpClient.R().
		SetQueryParam("secid", secid).
		SetQueryParam("fields", "f43,f57,f58,f59").
		SetResult(&resp).
		Get("http://push2.eastmoney.com/api/qt/stock/get")
	if err != nil {
		return nil, fmt.Errorf("eastmoney HK stock request failed: %w", err)
	}
	if r.IsError() {
		return nil, fmt.Errorf("eastmoney HK stock returned status %d", r.StatusCode())
	}

	if resp.RC != 0 || resp.Data == nil {
		return nil, fmt.Errorf("eastmoney no data for HK stock %s", symbol)
	}

	price := float64(resp.Data.F43) / math.Pow(10, float64(resp.Data.F59))

	slog.Info("eastmoney HK stock price fetched from API", "symbol", symbol, "price", price)
	return &marketsource.Quote{
		Symbol:           symbol,
		Name:             resp.Data.F58,
		Price:            price,
		OriginalPrice:    price,
		Currency:         "HKD",
		OriginalCurrency: "HKD",
	}, nil
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

func fetchFundQuote(code string) (*marketsource.Quote, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("eastmoney client not initialized, call eastmoney.Init() first")
	}

	// code is canonical like "001811.CNOF", extract base code for API
	queryCode := marketsource.NormalizeForSource(code, "FUND", "eastmoney")

	r, err := httpClient.R().
		Get(fmt.Sprintf("https://fundgz.1234567.com.cn/js/%s.js", queryCode))
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

	slog.Info("eastmoney fund price fetched from API", "code", code, "price", price)
	return &marketsource.Quote{
		Symbol:           code,
		Name:             resp.Name,
		Price:            price,
		OriginalPrice:    price,
		Currency:         "CNY",
		OriginalCurrency: "CNY",
	}, nil
}
