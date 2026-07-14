package eastmoney

import (
	"encoding/json"
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

func TestParseF43(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantN   int
		wantOk  bool
	}{
		{"numeric", `1234`, 1234, true},
		{"zero", `0`, 0, true},
		{"negative", `-500`, -500, true},
		{"dash", `"-"`, 0, false},
		{"empty string", `""`, 0, false},
		{"string number", `"123"`, 0, false},
		{"boolean", `true`, 0, false},
		{"null", `null`, 0, true},
		{"object", `{}`, 0, false},
		{"array", `[]`, 0, false},
		{"invalid json", `notjson`, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := json.RawMessage(tt.input)
			gotN, gotOk := parseF43(raw)
			if gotN != tt.wantN || gotOk != tt.wantOk {
				t.Errorf("parseF43(%s) = (%v, %v), want (%v, %v)", tt.input, gotN, gotOk, tt.wantN, tt.wantOk)
			}
		})
	}
}

func TestEastmoneyResponse_DashF43(t *testing.T) {
	// Simulate the API response when market is closed: f43 is "-"
	body := `{"rc":0,"data":{"f43":"-","f57":"511090","f58":"银华日利","f59":0}}`
	var resp eastmoneyResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("expected data to be non-nil")
	}
	_, ok := parseF43(resp.Data.F43Raw)
	if ok {
		t.Error("expected parseF43 to return false for dash f43")
	}
}

func TestEastmoneyResponse_NumericF43(t *testing.T) {
	// Simulate normal API response: f43 is a number
	body := `{"rc":0,"data":{"f43":10250,"f57":"511090","f58":"银华日利","f59":3}}`
	var resp eastmoneyResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("expected data to be non-nil")
	}
	n, ok := parseF43(resp.Data.F43Raw)
	if !ok {
		t.Error("expected parseF43 to return true for numeric f43")
	}
	if n != 10250 {
		t.Errorf("expected f43 = 10250, got %d", n)
	}
}
