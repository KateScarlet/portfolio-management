package yahoo

import (
	"testing"

	"portfolio-management/marketsource"
)

func TestFetchQuote_UsesNormalizeForSource(t *testing.T) {
	// Test that canonical symbols are properly converted to Yahoo format
	tests := []struct {
		symbol string
		market string
		want   string
	}{
		{"600519.SH", "CN", "600519.SS"},
		{"000001.SZ", "CN", "000001.SZ"},
		{"00700.HK", "HK", "00700.HK"},
		{"AAPL.US", "US", "AAPL"},
		{"BTC.CC", "CRYPTO", "BTC-USD"},
		{"GC.INTL", "COMMODITY_INTL", "GC=F"},
		{"001811.CNOF", "FUND", "001811.SZ"},
	}

	for _, tt := range tests {
		t.Run(tt.symbol, func(t *testing.T) {
			got := marketsource.NormalizeForSource(tt.symbol, tt.market, "yahoo")
			if got != tt.want {
				t.Errorf("NormalizeForSource(%q, %q, %q) = %q, want %q", tt.symbol, tt.market, "yahoo", got, tt.want)
			}
		})
	}
}
