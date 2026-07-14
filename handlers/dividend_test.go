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
	var holding models.Holding
	if err := db.First(&holding, "id = ?", holdingID).Error; err != nil {
		t.Fatal(err)
	}
	if !holding.Cost.Equal(decimal.NewFromInt(815)) {
		t.Fatalf("expected cash dividend to reduce net investment to 815, got %s", holding.Cost)
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
	if err := db.First(&holding, "id = ?", holdingID).Error; err != nil {
		t.Fatal(err)
	}
	if !holding.Cost.Equal(decimal.NewFromInt(900)) {
		t.Fatalf("expected deleting cash dividend to restore net investment to 900, got %s", holding.Cost)
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
	if !holding.Shares.Equal(decimal.NewFromInt(15)) || !holding.TotalDividends.Equal(decimal.NewFromInt(100)) || !holding.Cost.Equal(decimal.NewFromInt(900)) {
		t.Fatalf("unexpected holding after reinvestment: shares=%s dividends=%s netInvestment=%s", holding.Shares, holding.TotalDividends, holding.Cost)
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

func TestListDividends_FiltersByHoldingAndUser(t *testing.T) {
	db := setupTestDB(t)
	firstHoldingID := uuid.MustParse(createTestHoldingWithCurrency(t, db, "USD", 10, 100, 900))
	secondHoldingID := uuid.New()
	if err := db.Create(&models.Holding{
		ID: secondHoldingID, UserID: testUserID, PortfolioID: testPortfolioID,
		AssetId: "stocks", Symbol: "SECOND", Currency: "USD",
	}).Error; err != nil {
		t.Fatal(err)
	}
	otherUserID := uuid.New()
	baseDate := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	dividends := []models.Dividend{
		{
			ID: uuid.New(), UserID: testUserID, PortfolioID: testPortfolioID, HoldingID: firstHoldingID,
			Type: DividendTypeCash, GrossAmount: decimal.NewFromInt(10), NetAmount: decimal.NewFromInt(10),
			Currency: "USD", PaymentDate: baseDate, FundTxID: uuid.New(),
		},
		{
			ID: uuid.New(), UserID: testUserID, PortfolioID: testPortfolioID, HoldingID: secondHoldingID,
			Type: DividendTypeCash, GrossAmount: decimal.NewFromInt(20), NetAmount: decimal.NewFromInt(20),
			Currency: "USD", PaymentDate: baseDate.Add(time.Hour), FundTxID: uuid.New(),
		},
		{
			ID: uuid.New(), UserID: otherUserID, PortfolioID: testPortfolioID, HoldingID: firstHoldingID,
			Type: DividendTypeCash, GrossAmount: decimal.NewFromInt(30), NetAmount: decimal.NewFromInt(30),
			Currency: "USD", PaymentDate: baseDate.Add(2 * time.Hour), FundTxID: uuid.New(),
		},
	}
	if err := db.Create(&dividends).Error; err != nil {
		t.Fatal(err)
	}

	listCtx := newDividendCtx("GET", "", nil)
	listCtx.Request.SetRequestURI("/api/dividends")
	ListDividends(db)(context.Background(), listCtx)
	if listCtx.Response.StatusCode() != 200 {
		t.Fatalf("list: expected 200, got %d: %s", listCtx.Response.StatusCode(), listCtx.Response.Body())
	}
	var result []models.Dividend
	if err := json.Unmarshal(listCtx.Response.Body(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 2 || result[0].HoldingID != secondHoldingID {
		t.Fatalf("expected own dividends in descending order, got %+v", result)
	}

	filterCtx := newDividendCtx("GET", "", nil)
	filterCtx.Request.SetRequestURI("/api/dividends?holdingId=" + firstHoldingID.String())
	ListDividends(db)(context.Background(), filterCtx)
	if err := json.Unmarshal(filterCtx.Response.Body(), &result); err != nil {
		t.Fatal(err)
	}
	if filterCtx.Response.StatusCode() != 200 || len(result) != 1 || result[0].HoldingID != firstHoldingID {
		t.Fatalf("unexpected holding filter result: %+v", result)
	}

	invalidCtx := newDividendCtx("GET", "", nil)
	invalidCtx.Request.SetRequestURI("/api/dividends?holdingId=invalid")
	ListDividends(db)(context.Background(), invalidCtx)
	if invalidCtx.Response.StatusCode() != 400 {
		t.Fatalf("invalid holding filter: expected 400, got %d", invalidCtx.Response.StatusCode())
	}
}

func TestUpdateDividend_CashReinvestCashLifecycle(t *testing.T) {
	db := setupTestDB(t)
	holdingID := uuid.MustParse(createTestHoldingWithCurrency(t, db, "USD", 10, 100, 900))
	paymentDate := time.Now().Add(-24 * time.Hour).Truncate(time.Second)
	createCtx := newDividendCtx("POST", "", CreateDividendRequest{
		HoldingID: holdingID, GrossAmount: decimal.NewFromInt(110), TaxAmount: decimal.NewFromInt(10),
		Type: DividendTypeCash, PaymentDate: paymentDate, Note: "cash",
	})
	RecordDividend(db)(context.Background(), createCtx)
	if createCtx.Response.StatusCode() != 201 {
		t.Fatalf("create: expected 201, got %d: %s", createCtx.Response.StatusCode(), createCtx.Response.Body())
	}
	var dividend models.Dividend
	if err := json.Unmarshal(createCtx.Response.Body(), &dividend); err != nil {
		t.Fatal(err)
	}
	originalFundTxID := dividend.FundTxID

	reinvestCtx := newDividendCtx("PUT", dividend.ID.String(), UpdateDividendRequest{
		GrossAmount: decimal.NewFromInt(120), TaxAmount: decimal.NewFromInt(20),
		Type: DividendTypeReinvest, PaymentDate: paymentDate, ReinvestmentPrice: decimal.NewFromInt(20),
		Note: "reinvested",
	})
	UpdateDividend(db)(context.Background(), reinvestCtx)
	if reinvestCtx.Response.StatusCode() != 200 {
		t.Fatalf("cash to reinvest: expected 200, got %d: %s", reinvestCtx.Response.StatusCode(), reinvestCtx.Response.Body())
	}
	if err := json.Unmarshal(reinvestCtx.Response.Body(), &dividend); err != nil {
		t.Fatal(err)
	}
	if dividend.Type != DividendTypeReinvest || dividend.HoldingLotID == nil ||
		!dividend.ReinvestedShares.Equal(decimal.NewFromInt(5)) || dividend.FundTxID == originalFundTxID {
		t.Fatalf("unexpected reinvested dividend: %+v", dividend)
	}
	var holding models.Holding
	if err := db.First(&holding, "id = ?", holdingID).Error; err != nil {
		t.Fatal(err)
	}
	if !holding.Shares.Equal(decimal.NewFromInt(15)) || !holding.TotalDividends.Equal(decimal.NewFromInt(100)) || !holding.Cost.Equal(decimal.NewFromInt(900)) {
		t.Fatalf("unexpected holding after reinvestment: shares=%s dividends=%s netInvestment=%s", holding.Shares, holding.TotalDividends, holding.Cost)
	}
	var fund models.AvailableFund
	if err := db.Where("portfolio_id = ? AND currency = ?", testPortfolioID, "USD").First(&fund).Error; err != nil {
		t.Fatal(err)
	}
	if !fund.Amount.IsZero() {
		t.Fatalf("expected cash dividend removed from funds, got %s", fund.Amount)
	}

	cashCtx := newDividendCtx("PUT", dividend.ID.String(), UpdateDividendRequest{
		GrossAmount: decimal.NewFromInt(90), TaxAmount: decimal.NewFromInt(10),
		Type: DividendTypeCash, PaymentDate: paymentDate.Add(time.Hour), Note: "cash again",
	})
	UpdateDividend(db)(context.Background(), cashCtx)
	if cashCtx.Response.StatusCode() != 200 {
		t.Fatalf("reinvest to cash: expected 200, got %d: %s", cashCtx.Response.StatusCode(), cashCtx.Response.Body())
	}
	var cashDividend models.Dividend
	if err := json.Unmarshal(cashCtx.Response.Body(), &cashDividend); err != nil {
		t.Fatal(err)
	}
	if cashDividend.Type != DividendTypeCash || cashDividend.HoldingLotID != nil || !cashDividend.ReinvestedShares.IsZero() {
		t.Fatalf("unexpected cash dividend after update: %+v", cashDividend)
	}
	var persistedDividend models.Dividend
	if err := db.First(&persistedDividend, "id = ?", cashDividend.ID).Error; err != nil {
		t.Fatal(err)
	}
	if persistedDividend.HoldingLotID != nil {
		t.Fatalf("expected persisted holding lot reference cleared, got %s", *persistedDividend.HoldingLotID)
	}
	if err := db.First(&holding, "id = ?", holdingID).Error; err != nil {
		t.Fatal(err)
	}
	if !holding.Shares.Equal(decimal.NewFromInt(10)) || !holding.TotalDividends.Equal(decimal.NewFromInt(80)) || !holding.Cost.Equal(decimal.NewFromInt(820)) {
		t.Fatalf("unexpected holding after cash update: shares=%s dividends=%s netInvestment=%s", holding.Shares, holding.TotalDividends, holding.Cost)
	}
	if err := db.First(&fund, "id = ?", fund.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !fund.Amount.Equal(decimal.NewFromInt(80)) {
		t.Fatalf("expected 80 USD available funds, got %s", fund.Amount)
	}
	var txs []models.FundTransaction
	if err := db.Where("holding_id = ?", holdingID).Find(&txs).Error; err != nil {
		t.Fatal(err)
	}
	if len(txs) != 1 || txs[0].Type != "dividend_cash" || !txs[0].Amount.Equal(decimal.NewFromInt(80)) {
		t.Fatalf("expected one replacement cash transaction, got %+v", txs)
	}
}

func TestUpdateDividend_RejectsInvalidOrMissingRecord(t *testing.T) {
	db := setupTestDB(t)
	validBody := UpdateDividendRequest{
		GrossAmount: decimal.NewFromInt(10), Type: DividendTypeCash, PaymentDate: time.Now(),
	}

	invalidCtx := newDividendCtx("PUT", "invalid", validBody)
	UpdateDividend(db)(context.Background(), invalidCtx)
	if invalidCtx.Response.StatusCode() != 400 {
		t.Fatalf("invalid id: expected 400, got %d", invalidCtx.Response.StatusCode())
	}

	missingCtx := newDividendCtx("PUT", uuid.NewString(), validBody)
	UpdateDividend(db)(context.Background(), missingCtx)
	if missingCtx.Response.StatusCode() != 404 {
		t.Fatalf("missing record: expected 404, got %d: %s", missingCtx.Response.StatusCode(), missingCtx.Response.Body())
	}
}
