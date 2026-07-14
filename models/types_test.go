package models

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
)

func TestHolding_RecalcFromLots_SymbolBased(t *testing.T) {
	h := &Holding{
		Symbol: "VTI",
		Price:  decimal.NewFromInt(100),
	}
	lots := []HoldingLot{
		{Type: "", Shares: decimal.NewFromInt(10), Cost: decimal.NewFromInt(950), ValueAdded: decimal.NewFromInt(1000), Fee: decimal.NewFromInt(5)},
		{Type: "", Shares: decimal.NewFromInt(5), Cost: decimal.NewFromInt(490), ValueAdded: decimal.NewFromInt(500), Fee: decimal.NewFromInt(3)},
		{Type: "sell", Shares: decimal.NewFromInt(3), Cost: decimal.NewFromInt(285), ValueAdded: decimal.NewFromInt(300), Fee: decimal.NewFromInt(2)},
	}
	RecalcFromLots(h, lots)

	if !h.Shares.Equal(decimal.NewFromInt(12)) {
		t.Errorf("expected shares=12, got %s", h.Shares)
	}
	if !h.Cost.Equal(decimal.NewFromInt(1150)) {
		t.Errorf("expected net investment=1150, got %s", h.Cost)
	}
	expectedCostPrice := decimal.NewFromInt(1150).Div(decimal.NewFromInt(12))
	if !h.CostPrice.Equal(expectedCostPrice) {
		t.Errorf("expected costPrice=%s, got %s", expectedCostPrice, h.CostPrice)
	}
	if !h.Value.Equal(decimal.NewFromInt(1200)) {
		t.Errorf("expected value=1200, got %s", h.Value)
	}
	if !TotalFees(lots).Equal(decimal.NewFromInt(10)) {
		t.Errorf("expected totalFees=10, got %s", TotalFees(lots))
	}
	if !CostBasis(lots).Equal(decimal.NewFromInt(1155)) {
		t.Errorf("expected remaining cost basis=1155, got %s", CostBasis(lots))
	}
}

func TestHolding_RecalcFromLots_PreservesTinyNonZeroShares(t *testing.T) {
	tinyShares := decimal.RequireFromString("0.0000000001")
	h := &Holding{
		Symbol: "BTC",
		Price:  decimal.NewFromInt(100),
	}
	lots := []HoldingLot{
		{
			Type:       "",
			Shares:     tinyShares,
			Cost:       decimal.RequireFromString("0.000001"),
			ValueAdded: decimal.RequireFromString("0.00001"),
			Fee:        decimal.Zero,
		},
	}

	RecalcFromLots(h, lots)

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
	}
	lots := []HoldingLot{
		{Type: "", Shares: decimal.NewFromInt(10), Cost: decimal.NewFromInt(1000), ValueAdded: decimal.NewFromInt(1000), Fee: decimal.NewFromInt(5)},
		{Type: "sell", Shares: decimal.NewFromInt(10), Cost: decimal.NewFromInt(1000), ValueAdded: decimal.NewFromInt(1100), Fee: decimal.NewFromInt(5)},
	}
	RecalcFromLots(h, lots)

	if !h.Shares.IsZero() {
		t.Errorf("expected shares=0, got %s", h.Shares)
	}
	if !h.Cost.Equal(decimal.NewFromInt(-90)) {
		t.Errorf("expected net investment=-90, got %s", h.Cost)
	}
	if !h.Value.IsZero() {
		t.Errorf("expected value=0, got %s", h.Value)
	}
}

func TestHolding_RecalcFromLots_ManualHolding(t *testing.T) {
	h := &Holding{
		Symbol: "",
	}
	lots := []HoldingLot{
		{Type: "", Shares: decimal.Zero, Cost: decimal.NewFromInt(5000), ValueAdded: decimal.NewFromInt(5000), Fee: decimal.Zero},
		{Type: "", Shares: decimal.Zero, Cost: decimal.NewFromInt(3000), ValueAdded: decimal.NewFromInt(3000), Fee: decimal.Zero},
		{Type: "sell", Shares: decimal.Zero, Cost: decimal.NewFromInt(2000), ValueAdded: decimal.NewFromInt(2500), Fee: decimal.Zero},
	}
	RecalcFromLots(h, lots)

	if !h.Shares.IsZero() {
		t.Errorf("expected shares=0, got %s", h.Shares)
	}
	if !h.Value.Equal(decimal.NewFromInt(5500)) {
		t.Errorf("expected value=5500, got %s", h.Value)
	}
	if !h.Cost.Equal(decimal.NewFromInt(5500)) {
		t.Errorf("expected net investment=5500, got %s", h.Cost)
	}
}

func TestHolding_BuyFees(t *testing.T) {
	lots := []HoldingLot{
		{Type: "", Shares: decimal.NewFromInt(10), Cost: decimal.NewFromInt(950), Fee: decimal.NewFromInt(5)},
		{Type: "", Shares: decimal.NewFromInt(5), Cost: decimal.NewFromInt(490), Fee: decimal.NewFromInt(3)},
		{Type: "sell", Shares: decimal.NewFromInt(3), Cost: decimal.NewFromInt(285), Fee: decimal.NewFromInt(2)},
	}

	if !TotalFees(lots).Equal(decimal.NewFromInt(10)) {
		t.Errorf("expected totalFees=10, got %s", TotalFees(lots))
	}
	if !BuyFees(lots).Equal(decimal.NewFromInt(8)) {
		t.Errorf("expected buyFees=8, got %s", BuyFees(lots))
	}
	if !SellFees(lots).Equal(decimal.NewFromInt(2)) {
		t.Errorf("expected sellFees=2, got %s", SellFees(lots))
	}
}

func TestHolding_RecalcFromLots_Empty(t *testing.T) {
	h := &Holding{
		Shares: decimal.NewFromInt(10), Value: decimal.NewFromInt(100),
		Cost: decimal.NewFromInt(80), CostPrice: decimal.NewFromInt(8),
	}
	RecalcFromLots(h, nil)
	if !h.Shares.IsZero() || !h.Value.IsZero() || !h.Cost.IsZero() || !h.CostPrice.IsZero() {
		t.Fatalf("empty lots should reset calculated fields: %+v", h)
	}
}

func TestHolding_RecalcFromLots_DividendCashAndReinvestment(t *testing.T) {
	h := &Holding{
		Symbol:         "VTI",
		Price:          decimal.NewFromInt(100),
		TotalDividends: decimal.NewFromInt(150),
	}
	lots := []HoldingLot{
		{Type: "buy", Shares: decimal.NewFromInt(10), Cost: decimal.NewFromInt(1000), Fee: decimal.NewFromInt(5)},
		{Type: "buy", Source: "dividend_reinvest", Shares: decimal.NewFromInt(1), Cost: decimal.NewFromInt(100)},
	}

	RecalcFromLots(h, lots)

	// The 100 reinvested dividend is an internal transfer. Only the remaining
	// 50 cash dividend reduces the original 1005 cash outlay.
	if !h.Cost.Equal(decimal.NewFromInt(955)) {
		t.Fatalf("expected net investment=955, got %s", h.Cost)
	}
	if !CostBasis(lots).Equal(decimal.NewFromInt(1100)) {
		t.Fatalf("expected cost basis=1100, got %s", CostBasis(lots))
	}
}

func TestAssetMapColumn_ScanAndValue(t *testing.T) {
	var assets AssetMapColumn
	if err := assets.Scan(nil); err != nil || len(assets) != 0 {
		t.Fatalf("nil scan: assets=%v err=%v", assets, err)
	}
	if err := assets.Scan(`{"stocks":"12.5"}`); err != nil {
		t.Fatal(err)
	}
	if !assets["stocks"].Equal(decimal.RequireFromString("12.5")) {
		t.Fatalf("unexpected scanned assets: %+v", assets)
	}
	if err := assets.Scan([]byte(`{"cash":"7"}`)); err != nil {
		t.Fatal(err)
	}
	if !assets["cash"].Equal(decimal.NewFromInt(7)) {
		t.Fatalf("unexpected byte scan result: %+v", assets)
	}
	if err := assets.Scan(123); err == nil {
		t.Fatal("unsupported scan type should fail")
	}
	if err := assets.Scan("{"); err == nil {
		t.Fatal("invalid JSON should fail")
	}

	value, err := (AssetMapColumn{"bonds": decimal.NewFromInt(3)}).Value()
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]decimal.Decimal
	if err := json.Unmarshal(value.([]byte), &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded["bonds"].Equal(decimal.NewFromInt(3)) {
		t.Fatalf("unexpected serialized assets: %s", value)
	}
	nilValue, err := (AssetMapColumn(nil)).Value()
	if err != nil || nilValue != "{}" {
		t.Fatalf("nil assets should serialize as {}, got %v, err=%v", nilValue, err)
	}
}

func TestHoldingSnapshotColumn_ScanAndValue(t *testing.T) {
	var snapshots HoldingSnapshotColumn
	if err := snapshots.Scan(nil); err != nil || len(snapshots) != 0 {
		t.Fatalf("nil scan: snapshots=%v err=%v", snapshots, err)
	}
	payload := `[{"symbol":"AAPL","shares":"2","value":"300"}]`
	if err := snapshots.Scan(payload); err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 || snapshots[0].Symbol != "AAPL" || !snapshots[0].Shares.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("unexpected scanned snapshots: %+v", snapshots)
	}
	if err := snapshots.Scan([]byte(payload)); err != nil {
		t.Fatal(err)
	}
	if err := snapshots.Scan(true); err == nil {
		t.Fatal("unsupported scan type should fail")
	}
	if err := snapshots.Scan("["); err == nil {
		t.Fatal("invalid JSON should fail")
	}

	value, err := snapshots.Value()
	if err != nil {
		t.Fatal(err)
	}
	var decoded []HoldingSnapshot
	if err := json.Unmarshal(value.([]byte), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 1 || decoded[0].Symbol != "AAPL" {
		t.Fatalf("unexpected serialized snapshots: %s", value)
	}
	nilValue, err := (HoldingSnapshotColumn(nil)).Value()
	if err != nil || nilValue != "[]" {
		t.Fatalf("nil snapshots should serialize as [], got %v, err=%v", nilValue, err)
	}
}

func TestDividendTableName(t *testing.T) {
	if got := (Dividend{}).TableName(); got != "dividend_events" {
		t.Fatalf("expected dividend_events, got %q", got)
	}
}
