package handlers

import (
	"context"
	"encoding/json"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route/param"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func newDividendCtx(method, id string, body any) *app.RequestContext {
	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID.String()}}
	if id != "" {
		c.Params = append(c.Params, param.Param{Key: "id", Value: id})
	}
	c.Request.Header.SetMethod(method)
	c.Request.Header.SetContentTypeBytes([]byte("application/json"))
	b, _ := json.Marshal(body)
	c.Request.SetBodyRaw(b)
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{UserID: testUserID, Username: "testuser", Role: "user"})
	return c
}

func TestValidateDividendInput(t *testing.T) {
	valid := dividendInput{
		GrossAmount: decimal.NewFromInt(100), TaxAmount: decimal.NewFromInt(15),
		Type: DividendTypeCash, PaymentDate: time.Now(),
	}
	net, err := validateDividendInput(valid)
	if err != nil || !net.Equal(decimal.NewFromInt(85)) {
		t.Fatalf("expected net amount 85, got %s, error=%v", net, err)
	}

	cases := []dividendInput{
		{GrossAmount: decimal.Zero, Type: DividendTypeCash, PaymentDate: time.Now()},
		{GrossAmount: decimal.NewFromInt(10), TaxAmount: decimal.NewFromInt(-1), Type: DividendTypeCash, PaymentDate: time.Now()},
		{GrossAmount: decimal.NewFromInt(10), TaxAmount: decimal.NewFromInt(10), Type: DividendTypeCash, PaymentDate: time.Now()},
		{GrossAmount: decimal.NewFromInt(10), Type: "unknown", PaymentDate: time.Now()},
		{GrossAmount: decimal.NewFromInt(10), Type: DividendTypeCash},
		{GrossAmount: decimal.NewFromInt(10), Type: DividendTypeReinvest, PaymentDate: time.Now()},
	}
	for i, input := range cases {
		if _, err := validateDividendInput(input); err == nil {
			t.Errorf("case %d should fail", i)
		}
	}
}

func TestDividendCashLifecycle(t *testing.T) {
	db := setupTestDB(t)
	currency := "D" + uuid.NewString()[:8]
	holdingID := createTestHoldingWithCurrency(t, db, currency, 10, 100, 900)
	date := time.Now().Add(-24 * time.Hour).Truncate(time.Second)
	c := newDividendCtx("POST", "", CreateDividendRequest{
		HoldingID: uuid.MustParse(holdingID), GrossAmount: decimal.NewFromInt(100),
		TaxAmount: decimal.NewFromInt(15), Type: DividendTypeCash, PaymentDate: date,
	})
	RecordDividend(db)(context.Background(), c)
	if c.Response.StatusCode() != 201 {
		t.Fatalf("expected 201, got %d: %s", c.Response.StatusCode(), c.Response.Body())
	}
	var dividend models.Dividend
	if err := json.Unmarshal(c.Response.Body(), &dividend); err != nil {
		t.Fatal(err)
	}
	if dividend.Currency != currency || !dividend.NetAmount.Equal(decimal.NewFromInt(85)) {
		t.Fatalf("unexpected dividend: %+v", dividend)
	}
	var fund models.AvailableFund
	if err := db.Where("portfolio_id = ? AND currency = ?", testPortfolioID, currency).First(&fund).Error; err != nil {
		t.Fatal(err)
	}
	if !fund.Amount.Equal(decimal.NewFromInt(85)) {
		t.Fatalf("expected available fund 85, got %s", fund.Amount)
	}

	deleteCtx := newDividendCtx("DELETE", dividend.ID.String(), nil)
	DeleteDividend(db)(context.Background(), deleteCtx)
	if deleteCtx.Response.StatusCode() != 204 {
		t.Fatalf("expected 204, got %d: %s", deleteCtx.Response.StatusCode(), deleteCtx.Response.Body())
	}
	if err := db.First(&fund, "id = ?", fund.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !fund.Amount.IsZero() {
		t.Fatalf("expected available fund 0 after delete, got %s", fund.Amount)
	}
}

func TestDividendReinvestmentPersistsShares(t *testing.T) {
	db := setupTestDB(t)
	holdingID := createTestHoldingWithCurrency(t, db, "CNY", 10, 100, 900)
	c := newDividendCtx("POST", "", CreateDividendRequest{
		HoldingID: uuid.MustParse(holdingID), GrossAmount: decimal.NewFromInt(110),
		TaxAmount: decimal.NewFromInt(10), Type: DividendTypeReinvest,
		PaymentDate: time.Now().Add(time.Hour), ReinvestmentPrice: decimal.NewFromInt(20),
	})
	RecordDividend(db)(context.Background(), c)
	if c.Response.StatusCode() != 201 {
		t.Fatalf("expected 201, got %d: %s", c.Response.StatusCode(), c.Response.Body())
	}
	var dividend models.Dividend
	if err := json.Unmarshal(c.Response.Body(), &dividend); err != nil {
		t.Fatal(err)
	}
	if !dividend.ReinvestedShares.Equal(decimal.NewFromInt(5)) || dividend.HoldingLotID == nil {
		t.Fatalf("expected 5 persisted reinvested shares, got %+v", dividend)
	}
	var holding models.Holding
	if err := db.First(&holding, "id = ?", holdingID).Error; err != nil {
		t.Fatal(err)
	}
	if !holding.Shares.Equal(decimal.NewFromInt(15)) || !holding.TotalDividends.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("unexpected holding after reinvestment: shares=%s dividends=%s", holding.Shares, holding.TotalDividends)
	}
}

func TestDividendReinvestmentCannotBeDeletedAfterSale(t *testing.T) {
	db := setupTestDB(t)
	holdingID := uuid.MustParse(createTestHoldingWithCurrency(t, db, "CNY", 10, 100, 900))
	paymentDate := time.Now().Add(time.Hour)
	createCtx := newDividendCtx("POST", "", CreateDividendRequest{
		HoldingID: holdingID, GrossAmount: decimal.NewFromInt(100), Type: DividendTypeReinvest,
		PaymentDate: paymentDate, ReinvestmentPrice: decimal.NewFromInt(20),
	})
	RecordDividend(db)(context.Background(), createCtx)
	if createCtx.Response.StatusCode() != 201 {
		t.Fatalf("expected 201, got %d: %s", createCtx.Response.StatusCode(), createCtx.Response.Body())
	}
	var dividend models.Dividend
	if err := json.Unmarshal(createCtx.Response.Body(), &dividend); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.HoldingLot{
		ID: uuid.New(), HoldingID: holdingID, Type: "sell", Date: paymentDate.Add(time.Hour),
		Shares: decimal.NewFromInt(1), Cost: decimal.NewFromInt(20), ValueAdded: decimal.NewFromInt(20),
	}).Error; err != nil {
		t.Fatal(err)
	}

	deleteCtx := newDividendCtx("DELETE", dividend.ID.String(), nil)
	DeleteDividend(db)(context.Background(), deleteCtx)
	if deleteCtx.Response.StatusCode() != 409 {
		t.Fatalf("expected 409, got %d: %s", deleteCtx.Response.StatusCode(), deleteCtx.Response.Body())
	}
	if err := db.First(&dividend, "id = ?", dividend.ID).Error; err != nil {
		t.Fatal("rejected deletion must keep the dividend record")
	}
}
