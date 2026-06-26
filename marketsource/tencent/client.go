package tencent

import (
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"portfolio-management/marketsource"
)

var httpClient *resty.Client

func Init() {
	httpClient = resty.New().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetTimeout(10 * time.Second).
		SetRetryCount(2).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(3 * time.Second).
		AddRetryCondition(func(r *resty.Response, err error) bool {
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
	return []string{"CN", "HK"}
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
	if resp.IsError() {
		return nil, fmt.Errorf("tencent returned status %d", resp.StatusCode())
	}

	body := gbkToUTF8(resp.Body())
	return parseQuote(body, symbol)
}

func (c *Client) FetchExchangeRate(pair string) (float64, error) {
	return 0, marketsource.ErrNotSupported
}

func gbkToUTF8(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	reader := transform.NewReader(strings.NewReader(string(data)), simplifiedchinese.GBK.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return string(data)
	}
	return string(decoded)
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
	priceStr := fields[3]

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil || price <= 0 {
		return nil, fmt.Errorf("tencent invalid price for %s: %s", originalSymbol, priceStr)
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
