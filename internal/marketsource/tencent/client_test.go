package tencent

import (
	"testing"

	"portfolio-management/internal/marketsource"
)

func TestNormalizeForSource_Tencent(t *testing.T) {
	tests := []struct {
		symbol string
		market string
		want   string
	}{
		{"600519.SH", "CN", "sh600519"},
		{"000001.SZ", "CN", "sz000001"},
		{"0700.HK", "HK", "hk00700"},
		{"2800.HK", "HK", "hk02800"},
		{"001811.CNOF", "FUND", "jj001811"},
	}

	for _, tt := range tests {
		t.Run(tt.symbol, func(t *testing.T) {
			got := marketsource.NormalizeForSource(tt.symbol, tt.market, "tencent")
			if got != tt.want {
				t.Errorf("NormalizeForSource(%q, %q, %q) = %q, want %q", tt.symbol, tt.market, "tencent", got, tt.want)
			}
		})
	}
}

func TestParseQuote(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		symbol  string
		wantErr bool
	}{
		{
			name:   "valid A-share",
			body:   `v_sh600519="1~贵州茅台~600519~1800.00~1790.00~1795.00~12345~6000~6345~1800.00~10~1799.00~20~1798.00~30~1797.00~40~1796.00~50";`,
			symbol: "600519.SH",
		},
		{
			name:    "empty response",
			body:    "",
			symbol:  "600519.SH",
			wantErr: true,
		},
		{
			name:    "no match",
			body:    `v_pv_none_match="1";`,
			symbol:  "999999.SH",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := parseQuote(tt.body, tt.symbol)
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
			if !q.Price.IsPositive() {
				t.Errorf("expected positive price, got %s", q.Price)
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

func TestParseFundQuote(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		symbol  string
		wantErr bool
	}{
		{
			name:   "valid fund",
			body:   `v_jj001811="001811~中欧明睿新常态混合A~0.0000~0.0000~~4.4013~4.6523~0.6955~2026-07-03~";`,
			symbol: "001811.CNOF",
		},
		{
			name:    "empty response",
			body:    "",
			symbol:  "001811.CNOF",
			wantErr: true,
		},
		{
			name:    "no match",
			body:    `v_pv_none_match="1";`,
			symbol:  "999999.CNOF",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := parseFundQuote(tt.body, tt.symbol)
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
			if !q.Price.IsPositive() {
				t.Errorf("expected positive price, got %s", q.Price)
			}
			if q.Name == "" {
				t.Errorf("expected non-empty name")
			}
			if q.Symbol != tt.symbol {
				t.Errorf("expected symbol %q, got %q", tt.symbol, q.Symbol)
			}
			if q.Currency != "CNY" {
				t.Errorf("expected currency CNY, got %s", q.Currency)
			}
		})
	}
}
