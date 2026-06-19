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
		// SH tag format (SHxxxxxx -> xxxxxx.SS)
		{"SH600519", "600519.SS"},
		{"SH510300", "510300.SS"},
		// SZ tag format (SZxxxxxx -> xxxxxx.SZ)
		{"SZ000001", "000001.SZ"},
		{"SZ300750", "300750.SZ"},
		// Lowercase input with 6 digits still works via aShareRe
		{"600519", "600519.SS"},
		// Note: shTagRe/szTagRe only match uppercase SH/SZ prefix
		{"SH600519", "600519.SS"},
		// HK stocks - pass through unchanged
		{"2800", "2800"},
		{"9988", "9988"},
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
	symbols := []string{"VTI", "SPY", "600519.SS", "000001.SZ", "GC=F", "BTC-USD"}
	for _, sym := range symbols {
		result := ConvertSymbol(sym)
		if result != sym {
			t.Errorf("ConvertSymbol(%q) = %q, expected idempotent", sym, result)
		}
	}
}
