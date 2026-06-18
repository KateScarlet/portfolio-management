package yahoo

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-resty/resty/v2"
)

var (
	client    *resty.Client
	aShareRe  = regexp.MustCompile(`^\d{6}$`)
	shPrefixRe = regexp.MustCompile(`^[56]\d{5}$`)
	szPrefixRe = regexp.MustCompile(`^[0123]\d{5}$`)
	shTagRe   = regexp.MustCompile(`^SH\d{6}$`)
	szTagRe   = regexp.MustCompile(`^SZ\d{6}$`)
)

func Init() {
	client = resty.New().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetHeader("Accept", "application/json")
}

type YahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Symbol              string  `json:"symbol"`
				Currency            string  `json:"currency"`
				RegularMarketPrice  float64 `json:"regularMarketPrice"`
				ShortName           string  `json:"shortName"`
				LongName            string  `json:"longName"`
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
	if shTagRe.MatchString(symbol) {
		return strings.ToUpper(symbol[2:]) + ".SS"
	}
	if szTagRe.MatchString(symbol) {
		return strings.ToUpper(symbol[2:]) + ".SZ"
	}
	return s
}

func FetchQuote(symbol string) (*PriceResult, error) {
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

	if currency != "" && currency != "CNY" {
		fxSymbol := fmt.Sprintf("%sCNY=X", currency)
		var fxResult YahooChartResponse
		fxResp, err := client.R().
			SetQueryParam("range", "1d").
			SetQueryParam("interval", "1d").
			SetResult(&fxResult).
			Get(fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s", fxSymbol))
		if err == nil && !fxResp.IsError() && len(fxResult.Chart.Result) > 0 {
			rate := fxResult.Chart.Result[0].Meta.RegularMarketPrice
			if rate > 0 {
				price = price * rate
			}
		}
	}

	return &PriceResult{
		Symbol:           meta.Symbol,
		Name:             name,
		Price:            price,
		OriginalPrice:    originalPrice,
		Currency:         "CNY",
		OriginalCurrency: currency,
	}, nil
}

func FetchExchangeRate(pair string) (float64, error) {
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

	return rate, nil
}
