package tencent

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"resty.dev/v3"

	"portfolio-management/internal/marketsource"
)

var httpClient *resty.Client

func Init() {
	httpClient = resty.New().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetTimeout(10 * time.Second).
		SetRetryCount(2).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(3 * time.Second).
		AddRetryConditions(func(r *resty.Response, err error) bool {
			if err != nil {
				return true
			}
			status := r.StatusCode()
			return status == 429 || status >= 500
		})
}

type Client struct{}

func (c *Client) Name() string { return "腾讯财经" }

func (c *Client) SupportedMarkets() []string {
	return []string{"CN", "HK", "FUND"}
}

func (c *Client) FetchQuote(symbol, market string) (*marketsource.Quote, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("tencent client not initialized, call tencent.Init() first")
	}

	querySymbol := marketsource.NormalizeForSource(symbol, market, "tencent")

	resp, err := httpClient.R().
		Get(fmt.Sprintf("https://qt.gtimg.cn/q=%s", querySymbol))
	if err != nil {
		return nil, fmt.Errorf("tencent request failed: %w", err)
	}
	if resp.IsStatusFailure() {
		return nil, fmt.Errorf("tencent returned status %d", resp.StatusCode())
	}

	body := marketsource.GBKToUTF8(resp.Bytes())
	if market == "FUND" {
		return parseFundQuote(body, symbol)
	}
	return parseQuote(body, symbol)
}

func (c *Client) FetchExchangeRate(pair string) (decimal.Decimal, error) {
	return decimal.Zero, marketsource.ErrNotSupported
}

func parseQuote(body, originalSymbol string) (*marketsource.Quote, error) {
	body = strings.TrimSpace(body)
	if body == "" || body == "v_pv_none_match=\"1\";" {
		return nil, fmt.Errorf("tencent no data for symbol %s", originalSymbol)
	}

	// Format: v_sh600519="1~贵州茅台~600519~1800.00~..."
	parts := strings.SplitN(body, "=", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("tencent unexpected response format")
	}

	data := strings.Trim(parts[1], "\"")
	fields := strings.Split(data, "~")
	if len(fields) < 4 {
		return nil, fmt.Errorf("tencent response has too few fields for %s", originalSymbol)
	}

	name := fields[1]

	price, err := decimal.NewFromString(fields[3])
	if err != nil || !price.IsPositive() {
		return nil, fmt.Errorf("tencent invalid price for %s: %s", originalSymbol, fields[3])
	}

	currency := "CNY"
	if strings.HasSuffix(originalSymbol, ".HK") {
		currency = "HKD"
	}

	slog.Info("tencent price fetched", "symbol", originalSymbol, "price", price)
	return &marketsource.Quote{
		Symbol:           originalSymbol,
		Name:             name,
		Price:            price,
		OriginalPrice:    price,
		Currency:         currency,
		OriginalCurrency: currency,
	}, nil
}

func parseFundQuote(body, originalSymbol string) (*marketsource.Quote, error) {
	body = strings.TrimSpace(body)
	if body == "" || body == "v_pv_none_match=\"1\";" {
		return nil, fmt.Errorf("tencent no data for fund %s", originalSymbol)
	}

	// Format: v_jj001811="001811~中欧明睿新常态混合A~0.0000~0.0000~~4.4013~4.6523~0.6955~2026-07-03~"
	parts := strings.SplitN(body, "=", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("tencent unexpected response format")
	}

	data := strings.Trim(parts[1], "\"")
	fields := strings.Split(data, "~")
	if len(fields) < 9 {
		return nil, fmt.Errorf("tencent fund response has too few fields for %s", originalSymbol)
	}

	name := fields[1]

	// Unit NAV is at index 5
	price, err := decimal.NewFromString(fields[5])
	if err != nil || !price.IsPositive() {
		return nil, fmt.Errorf("tencent invalid NAV for fund %s: %s", originalSymbol, fields[5])
	}

	slog.Info("tencent fund NAV fetched", "symbol", originalSymbol, "price", price)
	return &marketsource.Quote{
		Symbol:           originalSymbol,
		Name:             name,
		Price:            price,
		OriginalPrice:    price,
		Currency:         "CNY",
		OriginalCurrency: "CNY",
	}, nil
}
