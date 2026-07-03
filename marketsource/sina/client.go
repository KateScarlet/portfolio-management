package sina

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/shopspring/decimal"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"portfolio-management/marketsource"
)

var httpClient *resty.Client

func Init() {
	httpClient = resty.New().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetHeader("Referer", "https://finance.sina.com.cn").
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

func (c *Client) Name() string { return "新浪财经" }

func (c *Client) SupportedMarkets() []string {
	return []string{"US", "CN", "HK"}
}

func (c *Client) FetchQuote(symbol, market string) (*marketsource.Quote, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("sina client not initialized, call sina.Init() first")
	}

	querySymbol := marketsource.NormalizeForSource(symbol, market, "sina")

	resp, err := httpClient.R().
		Get(fmt.Sprintf("https://hq.sinajs.cn/list=%s", querySymbol))
	if err != nil {
		return nil, fmt.Errorf("sina request failed: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("sina returned status %d", resp.StatusCode())
	}

	// Sina returns GBK-encoded data, convert to UTF-8
	body := gbkToUTF8(resp.Body())
	return parseQuote(body, symbol, market)
}

func (c *Client) FetchExchangeRate(pair string) (decimal.Decimal, error) {
	return decimal.Zero, marketsource.ErrNotSupported
}

func gbkToUTF8(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return string(data)
	}
	return string(decoded)
}

func parseQuote(body, originalSymbol, market string) (*marketsource.Quote, error) {
	body = strings.TrimSpace(body)
	if body == "" || body == "var hq_str_=\"\"" {
		return nil, fmt.Errorf("sina no data for symbol %s", originalSymbol)
	}

	// Format: var hq_str_sh600519="贵州茅台,1800.000,...";
	_, after, ok := strings.Cut(body, "=\"")
	if !ok {
		return nil, fmt.Errorf("sina unexpected response format")
	}

	data := after
	data = strings.TrimRight(data, "\";")
	if data == "" {
		return nil, fmt.Errorf("sina no data for symbol %s", originalSymbol)
	}

	fields := strings.Split(data, ",")
	if len(fields) < 4 {
		return nil, fmt.Errorf("sina response has too few fields for %s", originalSymbol)
	}

	var name string
	var price decimal.Decimal
	var currency string

	switch market {
	case "CN":
		if len(fields) < 4 {
			return nil, fmt.Errorf("sina A-share response too short")
		}
		name = fields[0]
		p, err := decimal.NewFromString(fields[3])
		if err != nil || !p.IsPositive() {
			return nil, fmt.Errorf("sina invalid A-share price for %s: %s", originalSymbol, fields[3])
		}
		price = p
		currency = "CNY"
	case "HK":
		if len(fields) < 7 {
			return nil, fmt.Errorf("sina HK response too short")
		}
		name = fields[1]
		p, err := decimal.NewFromString(fields[6])
		if err != nil || !p.IsPositive() {
			return nil, fmt.Errorf("sina invalid HK price for %s: %s", originalSymbol, fields[6])
		}
		price = p
		currency = "HKD"
	case "US":
		if len(fields) < 2 {
			return nil, fmt.Errorf("sina US response too short")
		}
		name = fields[0]
		p, err := decimal.NewFromString(fields[1])
		if err != nil || !p.IsPositive() {
			return nil, fmt.Errorf("sina invalid US price for %s: %s", originalSymbol, fields[1])
		}
		price = p
		currency = "USD"
	default:
		return nil, fmt.Errorf("sina unsupported market: %s", market)
	}

	slog.Info("sina price fetched", "symbol", originalSymbol, "price", price)
	return &marketsource.Quote{
		Symbol:           originalSymbol,
		Name:             name,
		Price:            price,
		OriginalPrice:    price,
		Currency:         currency,
		OriginalCurrency: currency,
	}, nil
}
