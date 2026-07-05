package models

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
)

func TestJSONColumn_ScanNil(t *testing.T) {
	var j JSONColumn
	if err := j.Scan(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(j) != 0 {
		t.Fatalf("expected empty column, got %d items", len(j))
	}
}

func TestJSONColumn_ScanBytes(t *testing.T) {
	var j JSONColumn
	data := `[{"id":"1","shares":10,"costPrice":100}]`
	if err := j.Scan([]byte(data)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(j) != 1 {
		t.Fatalf("expected 1 lot, got %d", len(j))
	}
	if j[0].ID != "1" {
		t.Fatalf("expected lot ID '1', got %q", j[0].ID)
	}
	if !j[0].Shares.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("expected shares 10, got %s", j[0].Shares)
	}
}

func TestJSONColumn_ScanInvalidType(t *testing.T) {
	var j JSONColumn
	err := j.Scan("not bytes")
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestJSONColumn_ValueNil(t *testing.T) {
	var j JSONColumn
	v, err := j.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "[]" {
		t.Fatalf("expected '[]', got %q", v)
	}
}

func TestJSONColumn_ValueWithData(t *testing.T) {
	j := JSONColumn{{ID: "1", Shares: decimal.NewFromInt(5)}}
	v, err := j.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bytes, ok := v.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", v)
	}
	var parsed []HoldingLot
	if err := json.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(parsed) != 1 || parsed[0].ID != "1" {
		t.Fatalf("unexpected data: %+v", parsed)
	}
}

func TestAssetMapColumn_ScanNil(t *testing.T) {
	var a AssetMapColumn
	if err := a.Scan(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(a) != 0 {
		t.Fatalf("expected empty map, got %d items", len(a))
	}
}

func TestAssetMapColumn_ScanBytes(t *testing.T) {
	var a AssetMapColumn
	data := `{"stocks":1000,"bonds":2000}`
	if err := a.Scan([]byte(data)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !a["stocks"].Equal(decimal.NewFromInt(1000)) || !a["bonds"].Equal(decimal.NewFromInt(2000)) {
		t.Fatalf("unexpected values: %+v", a)
	}
}

func TestAssetMapColumn_ValueNil(t *testing.T) {
	var a AssetMapColumn
	v, err := a.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "{}" {
		t.Fatalf("expected '{}', got %q", v)
	}
}

func TestAssetMapColumn_ValueWithData(t *testing.T) {
	a := AssetMapColumn{"stocks": decimal.NewFromInt(500), "commodities": decimal.NewFromInt(300)}
	v, err := a.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bytes, ok := v.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", v)
	}
	var parsed map[string]decimal.Decimal
	if err := json.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if !parsed["stocks"].Equal(decimal.NewFromInt(500)) || !parsed["commodities"].Equal(decimal.NewFromInt(300)) {
		t.Fatalf("unexpected data: %+v", parsed)
	}
}

func TestHolding_RecalcFromLots_SymbolBased(t *testing.T) {
	h := &Holding{
		Symbol: "VTI",
		Price:  decimal.NewFromInt(100),
		Lots: []HoldingLot{
			{Type: "", Shares: decimal.NewFromInt(10), Cost: decimal.NewFromInt(950), ValueAdded: decimal.NewFromInt(1000), Fee: decimal.NewFromInt(5)},
			{Type: "", Shares: decimal.NewFromInt(5), Cost: decimal.NewFromInt(490), ValueAdded: decimal.NewFromInt(500), Fee: decimal.NewFromInt(3)},
			{Type: "sell", Shares: decimal.NewFromInt(3), Cost: decimal.NewFromInt(285), ValueAdded: decimal.NewFromInt(300), Fee: decimal.NewFromInt(2)},
		},
	}
	h.RecalcFromLots()

	if !h.Shares.Equal(decimal.NewFromInt(12)) {
		t.Errorf("expected shares=12, got %s", h.Shares)
	}
	if !h.Cost.Equal(decimal.NewFromInt(1155)) {
		t.Errorf("expected cost=1155, got %s", h.Cost)
	}
	if !h.CostPrice.Equal(decimal.RequireFromString("96.25")) {
		t.Errorf("expected costPrice=96.25, got %s", h.CostPrice)
	}
	if !h.Value.Equal(decimal.NewFromInt(1200)) {
		t.Errorf("expected value=1200, got %s", h.Value)
	}
	if !h.TotalFees().Equal(decimal.NewFromInt(10)) {
		t.Errorf("expected totalFees=10, got %s", h.TotalFees())
	}
}

func TestHolding_RecalcFromLots_PreservesTinyNonZeroShares(t *testing.T) {
	tinyShares := decimal.RequireFromString("0.0000000001")
	h := &Holding{
		Symbol: "BTC",
		Price:  decimal.NewFromInt(100),
		Lots: []HoldingLot{
			{
				Type:       "",
				Shares:     tinyShares,
				Cost:       decimal.RequireFromString("0.000001"),
				ValueAdded: decimal.RequireFromString("0.00001"),
				Fee:        decimal.Zero,
			},
		},
	}

	h.RecalcFromLots()

	if !h.Shares.Equal(tinyShares) {
		t.Fatalf("expected tiny shares to be preserved, got %s", h.Shares)
	}
	if h.Value.IsZero() {
		t.Fatalf("expected tiny value to be preserved, got %s", h.Value)
	}
}

func TestHolding_RecalcFromLots_FullySold(t *testing.T) {
	h := &Holding{
		Symbol: "VTI",
		Price:  decimal.NewFromInt(100),
		Lots: []HoldingLot{
			{Type: "", Shares: decimal.NewFromInt(10), Cost: decimal.NewFromInt(1000), ValueAdded: decimal.NewFromInt(1000), Fee: decimal.NewFromInt(5)},
			{Type: "sell", Shares: decimal.NewFromInt(10), Cost: decimal.NewFromInt(1000), ValueAdded: decimal.NewFromInt(1100), Fee: decimal.NewFromInt(5)},
		},
	}
	h.RecalcFromLots()

	if !h.Shares.IsZero() {
		t.Errorf("expected shares=0, got %s", h.Shares)
	}
	if !h.Cost.IsZero() {
		t.Errorf("expected cost=0, got %s", h.Cost)
	}
	if !h.Value.IsZero() {
		t.Errorf("expected value=0, got %s", h.Value)
	}
}

func TestHolding_RecalcFromLots_ManualHolding(t *testing.T) {
	h := &Holding{
		Symbol: "",
		Lots: []HoldingLot{
			{Type: "", Shares: decimal.Zero, Cost: decimal.NewFromInt(5000), ValueAdded: decimal.NewFromInt(5000), Fee: decimal.Zero},
			{Type: "", Shares: decimal.Zero, Cost: decimal.NewFromInt(3000), ValueAdded: decimal.NewFromInt(3000), Fee: decimal.Zero},
			{Type: "sell", Shares: decimal.Zero, Cost: decimal.NewFromInt(2000), ValueAdded: decimal.NewFromInt(2500), Fee: decimal.Zero},
		},
	}
	h.RecalcFromLots()

	if !h.Shares.IsZero() {
		t.Errorf("expected shares=0, got %s", h.Shares)
	}
	if !h.Value.Equal(decimal.NewFromInt(5500)) {
		t.Errorf("expected value=5500, got %s", h.Value)
	}
	if !h.Cost.Equal(decimal.NewFromInt(6000)) {
		t.Errorf("expected cost=6000, got %s", h.Cost)
	}
}

func TestHolding_BuyFees(t *testing.T) {
	h := &Holding{
		Symbol: "VTI",
		Lots: []HoldingLot{
			{Type: "", Shares: decimal.NewFromInt(10), Cost: decimal.NewFromInt(950), Fee: decimal.NewFromInt(5)},
			{Type: "", Shares: decimal.NewFromInt(5), Cost: decimal.NewFromInt(490), Fee: decimal.NewFromInt(3)},
			{Type: "sell", Shares: decimal.NewFromInt(3), Cost: decimal.NewFromInt(285), Fee: decimal.NewFromInt(2)},
		},
	}

	if !h.TotalFees().Equal(decimal.NewFromInt(10)) {
		t.Errorf("expected totalFees=10, got %s", h.TotalFees())
	}
	if !h.BuyFees().Equal(decimal.NewFromInt(8)) {
		t.Errorf("expected buyFees=8, got %s", h.BuyFees())
	}
}
