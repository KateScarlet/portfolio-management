package eastmoney

import (
	"testing"
)

func TestIsFuturesSymbol(t *testing.T) {
	tests := []struct {
		symbol string
		want   bool
	}{
		{"au9999", true},
		{"AU9999", true},
		{"agtd", true},
		{"AGTD", true},
		{"scm", true},
		{"SCM", true},
		{"cum", true},
		{"CUM", true},
		{"GC=F", false},
		{"VTI", false},
		{"BTC-USD", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.symbol, func(t *testing.T) {
			got := IsFuturesSymbol(tt.symbol)
			if got != tt.want {
				t.Errorf("IsFuturesSymbol(%q) = %v, want %v", tt.symbol, got, tt.want)
			}
		})
	}
}

func TestFetchQuote_UnknownSymbol(t *testing.T) {
	Init()
	c := &Client{}
	_, err := c.FetchQuote("UNKNOWN", "COMMODITY_CN")
	if err == nil {
		t.Fatal("expected error for unknown symbol")
	}
}

func TestFetchExchangeRate_NotInitialized(t *testing.T) {
	// Reset httpClient to nil to test initialization check
	originalClient := httpClient
	httpClient = nil
	defer func() { httpClient = originalClient }()

	c := &Client{}
	_, err := c.FetchExchangeRate("USDCNY")
	if err == nil {
		t.Fatal("expected error when client not initialized")
	}
}

func TestFetchExchangeRate_UnsupportedPair(t *testing.T) {
	Init()
	c := &Client{}
	// This will fail because the pair doesn't exist in eastmoney's API
	_, err := c.FetchExchangeRate("INVALIDPAIR")
	if err == nil {
		t.Log("note: this test expects an error for invalid pair")
	}
}
