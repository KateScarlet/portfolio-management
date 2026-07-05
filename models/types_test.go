package models

import (
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
	if !h.Cost.Equal(decimal.NewFromInt(1155)) {
		t.Errorf("expected cost=1155, got %s", h.Cost)
	}
	if !h.CostPrice.Equal(decimal.RequireFromString("96.25")) {
		t.Errorf("expected costPrice=96.25, got %s", h.CostPrice)
	}
	if !h.Value.Equal(decimal.NewFromInt(1200)) {
		t.Errorf("expected value=1200, got %s", h.Value)
	}
	if !TotalFees(lots).Equal(decimal.NewFromInt(10)) {
		t.Errorf("expected totalFees=10, got %s", TotalFees(lots))
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
	if !h.Cost.Equal(decimal.NewFromInt(6000)) {
		t.Errorf("expected cost=6000, got %s", h.Cost)
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
}
