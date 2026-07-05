package marketsource

import (
	"bytes"
	"errors"
	"io"

	"github.com/shopspring/decimal"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var ErrNotSupported = errors.New("operation not supported by this source")

type Quote struct {
	Symbol           string          `json:"symbol"`
	Name             string          `json:"name"`
	Price            decimal.Decimal `json:"price"`
	OriginalPrice    decimal.Decimal `json:"originalPrice"`
	Currency         string          `json:"currency"`
	OriginalCurrency string          `json:"originalCurrency"`
	Unit             string          `json:"unit"`
}

// GBKToUTF8 converts GBK-encoded bytes to a UTF-8 string.
func GBKToUTF8(data []byte) string {
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
