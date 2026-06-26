package marketsource

import (
	"testing"
)

func TestNormalizeSymbol(t *testing.T) {
	tests := []struct {
		symbol string
		market string
		want   string
	}{
		// CN A-shares
		{"600519", "CN", "600519.SH"},
		{"000001", "CN", "000001.SZ"},
		{"sh600519", "CN", "600519.SH"},
		{"SH600519", "CN", "600519.SH"},
		{"sz000001", "CN", "000001.SZ"},
		{"SZ000001", "CN", "000001.SZ"},
		{"300750", "CN", "300750.SZ"},
		{"510300", "CN", "510300.SH"},
		{"600519.SH", "CN", "600519.SH"}, // already normalized

		// HK
		{"00700", "HK", "00700.HK"},
		{"2800", "HK", "02800.HK"},
		{"HK00700", "HK", "00700.HK"},
		{"00700.HK", "HK", "00700.HK"},

		// US
		{"AAPL", "US", "AAPL.US"},
		{"aapl", "US", "AAPL.US"},
		{"AAPL.US", "US", "AAPL.US"},

		// Crypto
		{"BTC", "CRYPTO", "BTC.CC"},
		{"btc", "CRYPTO", "BTC.CC"},
		{"BTC.CC", "CRYPTO", "BTC.CC"},
		{"BTC.USD", "CRYPTO", "BTC.CC"},
		{"BTC.USDT", "CRYPTO", "BTC.CC"},
		{"BTC-USD", "CRYPTO", "BTC.CC"},

		// Fund
		{"001811", "FUND", "001811.CNOF"},
		{"110011", "FUND", "110011.CNOF"},
		{"001811.CNOF", "FUND", "001811.CNOF"},

		// Commodity CN
		{"au9999", "COMMODITY_CN", "AU9999.CN"},
		{"AU9999", "COMMODITY_CN", "AU9999.CN"},
		{"au9999.CN", "COMMODITY_CN", "AU9999.CN"},

		// Commodity INTL
		{"GC", "COMMODITY_INTL", "GC.INTL"},
		{"gc", "COMMODITY_INTL", "GC.INTL"},
		{"GC.INTL", "COMMODITY_INTL", "GC.INTL"},
	}

	for _, tt := range tests {
		t.Run(tt.symbol+"_"+tt.market, func(t *testing.T) {
			got := NormalizeSymbol(tt.symbol, tt.market)
			if got != tt.want {
				t.Errorf("NormalizeSymbol(%q, %q) = %q, want %q", tt.symbol, tt.market, got, tt.want)
			}
		})
	}
}

func TestNormalizeForSource(t *testing.T) {
	tests := []struct {
		symbol string
		market string
		source string
		want   string
	}{
		// Yahoo
		{"600519.SH", "CN", "yahoo", "600519.SS"},
		{"000001.SZ", "CN", "yahoo", "000001.SZ"},
		{"00700.HK", "HK", "yahoo", "00700.HK"},
		{"AAPL.US", "US", "yahoo", "AAPL"},
		{"BTC.CC", "CRYPTO", "yahoo", "BTC-USD"},
		{"GC.INTL", "COMMODITY_INTL", "yahoo", "GC=F"},
		{"001811.CNOF", "FUND", "yahoo", "001811.SZ"},

		// Tencent
		{"600519.SH", "CN", "tencent", "sh600519"},
		{"000001.SZ", "CN", "tencent", "sz000001"},
		{"00700.HK", "HK", "tencent", "hk00700"},
		{"02800.HK", "HK", "tencent", "hk02800"},

		// Sina
		{"600519.SH", "CN", "sina", "sh600519"},
		{"000001.SZ", "CN", "sina", "sz000001"},
		{"00700.HK", "HK", "sina", "hk00700"},
		{"AAPL.US", "US", "sina", "gb_aapl"},

		// Eastmoney
		{"600519.SH", "CN", "eastmoney", "1.600519"},
		{"000001.SZ", "CN", "eastmoney", "0.000001"},
		{"au9999.CN", "COMMODITY_CN", "eastmoney", "au9999"},
		{"001811.CNOF", "FUND", "eastmoney", "001811"},
		{"AAPL.US", "US", "eastmoney", "105.AAPL"},
		{"00700.HK", "HK", "eastmoney", "116.00700"},

		// CoinGecko
		{"BTC.CC", "CRYPTO", "coingecko", "bitcoin"},
		{"ETH.CC", "CRYPTO", "coingecko", "ethereum"},
	}

	for _, tt := range tests {
		t.Run(tt.symbol+"_"+tt.source, func(t *testing.T) {
			got := NormalizeForSource(tt.symbol, tt.market, tt.source)
			if got != tt.want {
				t.Errorf("NormalizeForSource(%q, %q, %q) = %q, want %q", tt.symbol, tt.market, tt.source, got, tt.want)
			}
		})
	}
}

func TestExtractBaseSymbol(t *testing.T) {
	tests := []struct {
		symbol string
		want   string
	}{
		{"600519.SH", "600519"},
		{"AAPL.US", "AAPL"},
		{"BTC.CC", "BTC"},
		{"AAPL", "AAPL"},
	}

	for _, tt := range tests {
		t.Run(tt.symbol, func(t *testing.T) {
			got := ExtractBaseSymbol(tt.symbol)
			if got != tt.want {
				t.Errorf("ExtractBaseSymbol(%q) = %q, want %q", tt.symbol, got, tt.want)
			}
		})
	}
}
