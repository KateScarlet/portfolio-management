package sina

import (
	"testing"

	"portfolio-management/marketsource"
)

func TestNormalizeForSource_Sina(t *testing.T) {
	tests := []struct {
		symbol string
		market string
		want   string
	}{
		{"600519.SH", "CN", "sh600519"},
		{"000001.SZ", "CN", "sz000001"},
		{"00700.HK", "HK", "hk00700"},
		{"AAPL.US", "US", "gb_aapl"},
	}

	for _, tt := range tests {
		t.Run(tt.symbol, func(t *testing.T) {
			got := marketsource.NormalizeForSource(tt.symbol, tt.market, "sina")
			if got != tt.want {
				t.Errorf("NormalizeForSource(%q, %q, %q) = %q, want %q", tt.symbol, tt.market, "sina", got, tt.want)
			}
		})
	}
}

func TestParseQuote(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		symbol  string
		market  string
		wantErr bool
	}{
		{
			name:   "valid A-share",
			body:   `var hq_str_sh600519="贵州茅台,1800.000,1790.000,1800.000,1805.000,1795.000,1799.900,1800.000,1234567,2000000000.000,100,1799.900,200,1799.800,300,1799.700,400,1799.600,500,1799.500,2026-01-02,15:00:00,00";`,
			symbol: "600519.SH",
			market: "CN",
		},
		{
			name:   "valid HK",
			body:   `var hq_str_hk00700="TENCENT,腾讯控股,380.000,378.000,382.000,380.400,378.000,0.000,0,0,0";`,
			symbol: "00700.HK",
			market: "HK",
		},
		{
			name:   "valid US",
			body:   `var hq_str_gb_aapl="Apple Inc.,195.500,194.800,195.800";`,
			symbol: "AAPL.US",
			market: "US",
		},
		{
			name:    "empty response",
			body:    "",
			symbol:  "600519.SH",
			market:  "CN",
			wantErr: true,
		},
		{
			name:    "no match",
			body:    `var hq_str_=""`,
			symbol:  "999999.SH",
			market:  "CN",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := parseQuote(tt.body, tt.symbol, tt.market)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if q == nil {
				t.Errorf("expected quote, got nil")
				return
			}
			if q.Price <= 0 {
				t.Errorf("expected positive price, got %f", q.Price)
			}
			if q.Name == "" {
				t.Errorf("expected non-empty name")
			}
			if q.Symbol != tt.symbol {
				t.Errorf("expected symbol %q, got %q", tt.symbol, q.Symbol)
			}
		})
	}
}
