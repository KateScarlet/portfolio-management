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
