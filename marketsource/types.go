package marketsource

import "errors"

var ErrNotSupported = errors.New("operation not supported by this source")

type Quote struct {
	Symbol           string  `json:"symbol"`
	Name             string  `json:"name"`
	Price            float64 `json:"price"`
	OriginalPrice    float64 `json:"originalPrice"`
	Currency         string  `json:"currency"`
	OriginalCurrency string  `json:"originalCurrency"`
	Unit             string  `json:"unit"`
}
