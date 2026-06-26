package coingecko

import (
	"testing"

	"portfolio-management/marketsource"
)

func TestNormalizeForSource_CoinGecko(t *testing.T) {
	tests := []struct {
		symbol string
		market string
		want   string
	}{
		{"BTC.CC", "CRYPTO", "bitcoin"},
		{"ETH.CC", "CRYPTO", "ethereum"},
		{"BNB.CC", "CRYPTO", "binancecoin"},
		{"SOL.CC", "CRYPTO", "solana"},
	}

	for _, tt := range tests {
		t.Run(tt.symbol, func(t *testing.T) {
			got := marketsource.NormalizeForSource(tt.symbol, tt.market, "coingecko")
			if got != tt.want {
				t.Errorf("NormalizeForSource(%q, %q, %q) = %q, want %q", tt.symbol, tt.market, "coingecko", got, tt.want)
			}
		})
	}
}
