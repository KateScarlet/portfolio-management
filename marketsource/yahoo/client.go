package yahoo

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/go-resty/resty/v2"

	"portfolio-management/marketsource"
)

var httpClient *resty.Client

func Init() {
	httpClient = resty.New().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetHeader("Accept", "application/json").
		SetTimeout(10 * time.Second).
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

func (c *Client) Name() string { return "雅虎财经" }

func (c *Client) SupportedMarkets() []string {
	return []string{"US", "HK", "COMMODITY_INTL", "CRYPTO", "CN"}
}

func (c *Client) FetchQuote(symbol, market string) (*marketsource.Quote, error) {
	return fetchQuote(symbol, market)
}

func (c *Client) FetchExchangeRate(pair string) (float64, error) {
	return fetchExchangeRate(pair)
}

type yahooChartResponse struct {
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

func fetchQuote(symbol, market string) (*marketsource.Quote, error) {
	if httpClient == nil {
		return nil, fmt.Errorf("yahoo client not initialized, call yahoo.Init() first")
	}
	querySymbol := marketsource.NormalizeForSource(symbol, market, "yahoo")

	var result yahooChartResponse
	resp, err := httpClient.R().
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

	slog.Info("price fetched from API", "symbol", symbol, "querySymbol", querySymbol)
	return &marketsource.Quote{
		Symbol:           symbol,
		Name:             name,
		Price:            meta.RegularMarketPrice,
		OriginalPrice:    meta.RegularMarketPrice,
		Currency:         meta.Currency,
		OriginalCurrency: meta.Currency,
	}, nil
}

func fetchExchangeRate(pair string) (float64, error) {
	if httpClient == nil {
		return 0, fmt.Errorf("yahoo client not initialized, call yahoo.Init() first")
	}
	fxSymbol := pair + "=X"
	var result yahooChartResponse
	resp, err := httpClient.R().
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
	return rate, nil
}
