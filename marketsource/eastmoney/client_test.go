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
