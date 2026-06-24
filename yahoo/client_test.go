package yahoo

import "testing"

func TestConvertSymbol(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// US stocks - pass through unchanged (uppercase)
		{"VTI", "VTI"},
		{"spy", "SPY"},
		{"QQQ", "QQQ"},
		// A-share Shanghai (6xxxxx -> xxxxxx.SS)
		{"600519", "600519.SS"},
		{"601318", "601318.SS"},
		// A-share Shanghai (5xxxxx -> xxxxxx.SS)
		{"510300", "510300.SS"},
		{"510050", "510050.SS"},
		// A-share Shenzhen (0xxxxx, 1xxxxx, 2xxxxx, 3xxxxx -> xxxxxx.SZ)
		{"000001", "000001.SZ"},
		{"159915", "159915.SZ"},
		{"204001", "204001.SZ"},
		{"300750", "300750.SZ"},
		// SH tag format (SHxxxxxx -> xxxxxx.SS, case-insensitive)
		{"SH600519", "600519.SS"},
		{"SH510300", "510300.SS"},
		{"sh600519", "600519.SS"},
		{"Sh600519", "600519.SS"},
		// SZ tag format (SZxxxxxx -> xxxxxx.SZ, case-insensitive)
		{"SZ000001", "000001.SZ"},
		{"SZ300750", "300750.SZ"},
		{"sz000001", "000001.SZ"},
		{"Sz300750", "300750.SZ"},
		// HK stocks
		{"2800", "2800.HK"},
		{"9988", "9988.HK"},
		{"0700", "0700.HK"},
		// HK tag format (HKxxxx -> xxxx.HK)
		{"HK2800", "2800.HK"},
		{"HK0700", "0700.HK"},
		// Crypto - pass through unchanged
		{"BTC-USD", "BTC-USD"},
		{"ETH-USD", "ETH-USD"},
		// Commodities - pass through unchanged
		{"GC=F", "GC=F"},
		{"CL=F", "CL=F"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ConvertSymbol(tt.input)
			if result != tt.expected {
				t.Errorf("ConvertSymbol(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConvertSymbol_Idempotent(t *testing.T) {
	symbols := []string{"VTI", "SPY", "600519.SS", "000001.SZ", "2800.HK", "GC=F", "BTC-USD"}
	for _, sym := range symbols {
		result := ConvertSymbol(sym)
		if result != sym {
			t.Errorf("ConvertSymbol(%q) = %q, expected idempotent", sym, result)
		}
	}
}
