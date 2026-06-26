package coingecko

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

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

func (c *Client) Name() string { return "CoinGecko" }

func (c *Client) SupportedMarkets() []string {
	return []string{"CRYPTO"}
}

func (c *Client) FetchQuote(symbol, market string) (*marketsource.Quote, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("coingecko client not initialized, call coingecko.Init() first")
	}

	// symbol is canonical like "BTC.USD", normalize for source
	id := marketsource.NormalizeForSource(symbol, market, "coingecko")

	var result map[string]map[string]float64
	resp, err := httpClient.R().
		SetQueryParam("ids", id).
		SetQueryParam("vs_currencies", "usd").
		SetResult(&result).
		Get("https://api.coingecko.com/api/v3/simple/price")
	if err != nil {
		return nil, fmt.Errorf("coingecko request failed: %w", err)
	}
	if resp.IsError() {
		return nil, fmt.Errorf("coingecko returned status %d", resp.StatusCode())
	}

	prices, ok := result[id]
	if !ok {
		return nil, fmt.Errorf("coingecko no data for %s", symbol)
	}

	price, ok := prices["usd"]
	if !ok || price <= 0 {
		return nil, fmt.Errorf("coingecko invalid price for %s", symbol)
	}

	slog.Info("coingecko price fetched", "symbol", symbol, "price", price)
	return &marketsource.Quote{
		Symbol:           symbol,
		Name:             strings.ToUpper(marketsource.ExtractBaseSymbol(symbol)),
		Price:            price,
		OriginalPrice:    price,
		Currency:         "USD",
		OriginalCurrency: "USD",
	}, nil
}

func (c *Client) FetchExchangeRate(pair string) (float64, error) {
	return 0, marketsource.ErrNotSupported
}
