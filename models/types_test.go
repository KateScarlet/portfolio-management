package models

import (
	"encoding/json"
	"testing"
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
	if j[0].Shares != 10 {
		t.Fatalf("expected shares 10, got %f", j[0].Shares)
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
	j := JSONColumn{{ID: "1", Shares: 5}}
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
	if a["stocks"] != 1000 || a["bonds"] != 2000 {
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
	a := AssetMapColumn{"stocks": 500, "gold": 300}
	v, err := a.Value()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bytes, ok := v.([]byte)
	if !ok {
		t.Fatalf("expected []byte, got %T", v)
	}
	var parsed map[string]float64
	if err := json.Unmarshal(bytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if parsed["stocks"] != 500 || parsed["gold"] != 300 {
		t.Fatalf("unexpected data: %+v", parsed)
	}
}

func TestHolding_RecalcFromLots_SymbolBased(t *testing.T) {
	h := &Holding{
		Symbol: "VTI",
		Price:  100,
		Lots: []HoldingLot{
			{Type: "", Shares: 10, Cost: 950, ValueAdded: 1000, Fee: 5},
			{Type: "", Shares: 5, Cost: 490, ValueAdded: 500, Fee: 3},
			{Type: "sell", Shares: 3, Cost: 285, ValueAdded: 300, Fee: 2},
		},
	}
	h.RecalcFromLots()

	if h.Shares != 12 {
		t.Errorf("expected shares=12, got %f", h.Shares)
	}
	if h.Cost != 1155 {
		t.Errorf("expected cost=1155, got %f", h.Cost)
	}
	if h.CostPrice != 96.25 {
		t.Errorf("expected costPrice=96.25, got %f", h.CostPrice)
	}
	if h.Value != 1200 {
		t.Errorf("expected value=1200, got %f", h.Value)
	}
	if h.TotalFees() != 10 {
		t.Errorf("expected totalFees=10, got %f", h.TotalFees())
	}
}

func TestHolding_RecalcFromLots_FullySold(t *testing.T) {
	h := &Holding{
		Symbol: "VTI",
		Price:  100,
		Lots: []HoldingLot{
			{Type: "", Shares: 10, Cost: 1000, ValueAdded: 1000, Fee: 5},
			{Type: "sell", Shares: 10, Cost: 1000, ValueAdded: 1100, Fee: 5},
		},
	}
	h.RecalcFromLots()

	if h.Shares != 0 {
		t.Errorf("expected shares=0, got %f", h.Shares)
	}
	if h.Cost != 0 {
		t.Errorf("expected cost=0, got %f", h.Cost)
	}
	if h.Value != 0 {
		t.Errorf("expected value=0, got %f", h.Value)
	}
}

func TestHolding_RecalcFromLots_ManualHolding(t *testing.T) {
	h := &Holding{
		Symbol: "",
		Lots: []HoldingLot{
			{Type: "", Shares: 0, Cost: 5000, ValueAdded: 5000, Fee: 0},
			{Type: "", Shares: 0, Cost: 3000, ValueAdded: 3000, Fee: 0},
			{Type: "sell", Shares: 0, Cost: 2000, ValueAdded: 2500, Fee: 0},
		},
	}
	h.RecalcFromLots()

	if h.Shares != 0 {
		t.Errorf("expected shares=0, got %f", h.Shares)
	}
	if h.Value != 5500 {
		t.Errorf("expected value=5500, got %f", h.Value)
	}
	if h.Cost != 6000 {
		t.Errorf("expected cost=6000, got %f", h.Cost)
	}
}
