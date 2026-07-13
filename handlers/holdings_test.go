package handlers

import (
	"context"
	"encoding/json"
	"portfolio-management/internal/marketsource"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route/param"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func setupHoldingsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return setupTestDB(t)
}

func testRouter() *marketsource.Router {
	return marketsource.NewRouter(nil, map[string]marketsource.MarketSource{})
}

func newUserCtx(method, path string, body any) *app.RequestContext {
	c := app.NewContext(1)
	c.Request.SetRequestURI(path)
	c.Request.Header.SetMethod(method)
	c.Request.Header.SetContentTypeBytes([]byte("application/json"))
	if body != nil {
		b, _ := json.Marshal(body)
		c.Request.SetBodyRaw(b)
	}
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID.String()}}
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID:   testUserID,
		Username: "testuser",
		Role:     "user",
	})
	return c
}

// --- ListHoldings ---

func TestListHoldings_Empty(t *testing.T) {
	db := setupHoldingsTestDB(t)
	c := newUserCtx("GET", "/api/holdings", nil)

	ListHoldings(db, testRouter())(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", c.Response.StatusCode())
	}
	var holdings []models.Holding
	if err := json.Unmarshal(c.Response.Body(), &holdings); err != nil {
		t.Fatal(err)
	}
	if len(holdings) != 0 {
		t.Errorf("expected 0 holdings, got %d", len(holdings))
	}
}

func TestListHoldings_ReturnsUserHoldings(t *testing.T) {
	db := setupHoldingsTestDB(t)
	createTestHolding(t, db, 10, 100, 900)

	c := newUserCtx("GET", "/api/holdings", nil)
	ListHoldings(db, testRouter())(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", c.Response.StatusCode())
	}
	var holdings []models.Holding
	if err := json.Unmarshal(c.Response.Body(), &holdings); err != nil {
		t.Fatal(err)
	}
	if len(holdings) != 1 {
		t.Fatalf("expected 1 holding, got %d", len(holdings))
	}
	if holdings[0].Symbol != "TEST" {
		t.Errorf("expected symbol TEST, got %q", holdings[0].Symbol)
	}
}

func TestListHoldings_OtherUserNotReturned(t *testing.T) {
	db := setupHoldingsTestDB(t)
	createTestHolding(t, db, 10, 100, 900)

	otherPortfolioID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	otherUserID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	db.Create(&models.Portfolio{ID: otherPortfolioID, UserID: otherUserID, Name: "Other", IsDefault: true})

	c := app.NewContext(1)
	c.Request.SetRequestURI("/api/portfolios/" + otherPortfolioID.String() + "/holdings")
	c.Request.Header.SetMethod("GET")
	c.Params = param.Params{{Key: "pid", Value: otherPortfolioID.String()}}
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID:   otherUserID,
		Username: "other",
		Role:     "user",
	})

	ListHoldings(db, testRouter())(context.Background(), c)

	var holdings []models.Holding
	if err := json.Unmarshal(c.Response.Body(), &holdings); err != nil {
		t.Fatal(err)
	}
	if len(holdings) != 0 {
		t.Errorf("expected 0 holdings for other user, got %d", len(holdings))
	}
}

func TestListHoldings_Unauthorized(t *testing.T) {
	db := setupHoldingsTestDB(t)
	c := app.NewContext(1)
	c.Request.SetRequestURI("/api/holdings")
	c.Request.Header.SetMethod("GET")

	ListHoldings(db, testRouter())(context.Background(), c)

	if c.Response.StatusCode() != 401 {
		t.Errorf("expected 401, got %d", c.Response.StatusCode())
	}
}

// --- CreateHolding ---

func TestCreateHolding_NewStockHolding(t *testing.T) {
	db := setupHoldingsTestDB(t)
	db.Create(&models.AvailableFund{
		ID: uuid.New(), UserID: testUserID, PortfolioID: testPortfolioID,
		Currency: "CNY", Amount: decimal.NewFromInt(10000),
	})
	body := map[string]any{
		"assetId":   "stocks",
		"symbol":    "AAPL",
		"name":      "Apple",
		"shares":    10,
		"price":     150,
		"costPrice": 140,
		"cost":      1400,
		"value":     1500,
	}
	c := newUserCtx("POST", "/api/holdings", body)

	CreateHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 201 {
		t.Fatalf("expected 201, got %d: %s", c.Response.StatusCode(), string(c.Response.Body()))
	}
	var holding models.Holding
	if err := json.Unmarshal(c.Response.Body(), &holding); err != nil {
		t.Fatal(err)
	}
	if holding.Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %q", holding.Symbol)
	}
	if !holding.Shares.Equal(decimal.NewFromInt(10)) {
		t.Errorf("expected shares 10, got %s", holding.Shares)
	}
}

func TestCreateHolding_MergesIntoExisting(t *testing.T) {
	db := setupHoldingsTestDB(t)
	db.Create(&models.AvailableFund{
		ID: uuid.New(), UserID: testUserID, PortfolioID: testPortfolioID,
		Currency: "CNY", Amount: decimal.NewFromInt(10000),
	})
	createTestHolding(t, db, 10, 100, 900)

	body := map[string]any{
		"assetId":   "stocks",
		"symbol":    "TEST",
		"name":      "Test Stock",
		"shares":    5,
		"price":     110,
		"costPrice": 105,
		"cost":      525,
		"value":     550,
	}
	c := newUserCtx("POST", "/api/holdings", body)
	CreateHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200 (merge), got %d: %s", c.Response.StatusCode(), string(c.Response.Body()))
	}
	var resp HoldingResponse
	if err := json.Unmarshal(c.Response.Body(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Shares.Equal(decimal.NewFromInt(15)) {
		t.Errorf("expected merged shares 15, got %s", resp.Shares)
	}
	if len(resp.Lots) != 2 {
		t.Errorf("expected 2 lots, got %d", len(resp.Lots))
	}
}

func TestCreateHolding_ValidationErrors(t *testing.T) {
	db := setupHoldingsTestDB(t)

	tests := []struct {
		name string
		body map[string]any
		want int
	}{
		{"missing assetId", map[string]any{"symbol": "X"}, 400},
		{"invalid assetId", map[string]any{"assetId": "invalid"}, 400},
		{"negative shares", map[string]any{"assetId": "stocks", "shares": -1}, 400},
		{"negative cost", map[string]any{"assetId": "stocks", "cost": -1}, 400},
		{"negative costPrice", map[string]any{"assetId": "stocks", "costPrice": -1}, 400},
		{"negative fee", map[string]any{"assetId": "stocks", "fee": -1}, 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newUserCtx("POST", "/api/holdings", tt.body)
			CreateHolding(db)(context.Background(), c)
			if c.Response.StatusCode() != tt.want {
				t.Errorf("expected %d, got %d", tt.want, c.Response.StatusCode())
			}
		})
	}
}

func TestCreateHolding_DeductFromCash(t *testing.T) {
	db := setupHoldingsTestDB(t)
	db.Create(&models.AvailableFund{
		ID:          uuid.New(),
		UserID:      testUserID,
		PortfolioID: testPortfolioID,
		Currency:    "CNY",
		Amount:      decimal.NewFromInt(10000),
	})

	body := map[string]any{
		"assetId":   "stocks",
		"symbol":    "VTI",
		"shares":    10,
		"price":     100,
		"costPrice": 100,
		"cost":      1000,
		"value":     1000,
		"fee":       5,
		"currency":  "CNY",
	}
	c := newUserCtx("POST", "/api/holdings", body)
	CreateHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 201 {
		t.Fatalf("expected 201, got %d: %s", c.Response.StatusCode(), string(c.Response.Body()))
	}

	var af models.AvailableFund
	db.Where("user_id = ? AND portfolio_id = ? AND currency = ?", testUserID, testPortfolioID, "CNY").First(&af)
	expected := decimal.NewFromInt(8995) // 10000 - 1000 - 5
	if !af.Amount.Equal(expected) {
		t.Errorf("expected funds %s, got %s", expected, af.Amount)
	}
}

func TestCreateHolding_DeductFromCash_InsufficientFunds(t *testing.T) {
	db := setupHoldingsTestDB(t)
	db.Create(&models.AvailableFund{
		ID:          uuid.New(),
		UserID:      testUserID,
		PortfolioID: testPortfolioID,
		Currency:    "CNY",
		Amount:      decimal.NewFromInt(500),
	})

	body := map[string]any{
		"assetId":   "stocks",
		"symbol":    "VTI",
		"shares":    10,
		"price":     100,
		"costPrice": 100,
		"cost":      1000,
		"value":     1000,
	}
	c := newUserCtx("POST", "/api/holdings", body)
	CreateHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 400 {
		t.Errorf("expected 400 (insufficient funds), got %d", c.Response.StatusCode())
	}
}

func TestCreateHolding_DeductFromCash_USD(t *testing.T) {
	db := setupHoldingsTestDB(t)
	db.Create(&models.AvailableFund{
		ID: uuid.New(), UserID: testUserID, PortfolioID: testPortfolioID,
		Currency: "USD", Amount: decimal.NewFromInt(5000),
	})

	body := map[string]any{
		"assetId":   "stocks",
		"symbol":    "SPY",
		"shares":    5,
		"price":     500,
		"costPrice": 480,
		"cost":      2400,
		"value":     2500,
		"fee":       10,
		"currency":  "USD",
	}
	c := newUserCtx("POST", "/api/holdings", body)
	CreateHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 201 {
		t.Fatalf("expected 201, got %d: %s", c.Response.StatusCode(), string(c.Response.Body()))
	}

	var af models.AvailableFund
	db.Where("user_id = ? AND portfolio_id = ? AND currency = ?", testUserID, testPortfolioID, "USD").First(&af)
	expected := decimal.NewFromInt(2590) // 5000 - 2400 - 10
	if !af.Amount.Equal(expected) {
		t.Errorf("expected USD funds %s, got %s", expected, af.Amount)
	}

	var cnCount int64
	db.Model(&models.AvailableFund{}).Where("user_id = ? AND portfolio_id = ? AND currency = ?", testUserID, testPortfolioID, "CNY").Count(&cnCount)
	if cnCount != 0 {
		t.Errorf("expected no CNY fund affected, but found %d", cnCount)
	}
}

func TestCreateHolding_DeductFromCash_USD_Insufficient(t *testing.T) {
	db := setupHoldingsTestDB(t)
	db.Create(&models.AvailableFund{
		ID: uuid.New(), UserID: testUserID, PortfolioID: testPortfolioID,
		Currency: "USD", Amount: decimal.NewFromInt(100),
	})

	body := map[string]any{
		"assetId":   "stocks",
		"symbol":    "SPY",
		"shares":    5,
		"price":     500,
		"costPrice": 480,
		"cost":      2400,
		"value":     2500,
		"fee":       10,
		"currency":  "USD",
	}
	c := newUserCtx("POST", "/api/holdings", body)
	CreateHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 400 {
		t.Errorf("expected 400 (insufficient USD funds), got %d", c.Response.StatusCode())
	}
}

// --- UpdateHolding ---

func TestUpdateHolding_ManualValueUpdate(t *testing.T) {
	db := setupHoldingsTestDB(t)
	id := uuid.New()
	h := models.Holding{
		ID:          id,
		UserID:      testUserID,
		PortfolioID: testPortfolioID,
		AssetId:     "bonds",
		Name:        "手工债券",
		Value:       decimal.NewFromInt(5000),
		Cost:        decimal.NewFromInt(5000),
	}
	db.Create(&h)

	lot := models.HoldingLot{
		ID:         uuid.New(),
		HoldingID:  id,
		Date:       time.UnixMilli(1000),
		Cost:       decimal.NewFromInt(5000),
		ValueAdded: decimal.NewFromInt(5000),
	}
	db.Create(&lot)

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID.String()}, {Key: "id", Value: id.String()}}
	c.Request.SetRequestURI("/api/portfolios/" + testPortfolioID.String() + "/holdings/" + id.String())
	c.Request.Header.SetMethod("PATCH")
	c.Request.Header.SetContentTypeBytes([]byte("application/json"))
	c.Request.SetBodyRaw([]byte(`{"value": 6000}`))
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID:   testUserID,
		Username: "testuser",
		Role:     "user",
	})

	UpdateHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d: %s", c.Response.StatusCode(), string(c.Response.Body()))
	}

	var updated HoldingResponse
	if err := json.Unmarshal(c.Response.Body(), &updated); err != nil {
		t.Fatal(err)
	}
	if !updated.Value.Equal(decimal.NewFromInt(6000)) {
		t.Errorf("expected value 6000, got %s (json.Number fix)", updated.Value)
	}
}

func TestUpdateHolding_BlockAssetIdChange(t *testing.T) {
	db := setupHoldingsTestDB(t)
	id := createTestHolding(t, db, 10, 100, 900)

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID.String()}, {Key: "id", Value: id}}
	c.Request.SetRequestURI("/api/portfolios/" + testPortfolioID.String() + "/holdings/" + id)
	c.Request.Header.SetMethod("PATCH")
	c.Request.Header.SetContentTypeBytes([]byte("application/json"))
	c.Request.SetBodyRaw([]byte(`{"assetId": "bonds"}`))
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID:   testUserID,
		Username: "testuser",
		Role:     "user",
	})

	UpdateHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 400 {
		t.Errorf("expected 400, got %d", c.Response.StatusCode())
	}
}

func TestUpdateHolding_LotsRecalculation(t *testing.T) {
	db := setupHoldingsTestDB(t)
	id := createTestHolding(t, db, 10, 100, 900)

	newLots := []map[string]any{
		{"id": uuid.New(), "date": "1970-01-01T00:00:01Z", "shares": 20, "costPrice": 50, "cost": 1000, "valueAdded": 2000},
	}
	body := map[string]any{"lots": newLots}

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID.String()}, {Key: "id", Value: id}}
	c.Request.SetRequestURI("/api/portfolios/" + testPortfolioID.String() + "/holdings/" + id)
	c.Request.Header.SetMethod("PATCH")
	c.Request.Header.SetContentTypeBytes([]byte("application/json"))
	b, _ := json.Marshal(body)
	c.Request.SetBodyRaw(b)
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID:   testUserID,
		Username: "testuser",
		Role:     "user",
	})

	UpdateHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d: %s", c.Response.StatusCode(), string(c.Response.Body()))
	}

	var updated HoldingResponse
	if err := json.Unmarshal(c.Response.Body(), &updated); err != nil {
		t.Fatal(err)
	}
	if !updated.Shares.Equal(decimal.NewFromInt(20)) {
		t.Errorf("expected shares 20, got %s", updated.Shares)
	}
	if !updated.Cost.Equal(decimal.NewFromInt(1000)) {
		t.Errorf("expected cost 1000, got %s", updated.Cost)
	}
}

func TestUpdateHolding_NotFound(t *testing.T) {
	db := setupHoldingsTestDB(t)

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID.String()}, {Key: "id", Value: "nonexistent"}}
	c.Request.SetRequestURI("/api/portfolios/" + testPortfolioID.String() + "/holdings/nonexistent")
	c.Request.Header.SetMethod("PATCH")
	c.Request.Header.SetContentTypeBytes([]byte("application/json"))
	c.Request.SetBodyRaw([]byte(`{"name": "new"}`))
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID:   testUserID,
		Username: "testuser",
		Role:     "user",
	})

	UpdateHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 404 {
		t.Errorf("expected 404, got %d", c.Response.StatusCode())
	}
}

// --- DeleteHolding ---

func TestDeleteHolding_Success(t *testing.T) {
	db := setupHoldingsTestDB(t)
	id := createTestHolding(t, db, 10, 100, 900)

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID.String()}, {Key: "id", Value: id}}
	c.Request.SetRequestURI("/api/portfolios/" + testPortfolioID.String() + "/holdings/" + id)
	c.Request.Header.SetMethod("DELETE")
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID:   testUserID,
		Username: "testuser",
		Role:     "user",
	})

	DeleteHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", c.Response.StatusCode())
	}

	var count int64
	db.Model(&models.Holding{}).Where("id = ?", id).Count(&count)
	if count != 0 {
		t.Error("expected holding to be deleted")
	}
}

func TestDeleteHolding_NotFound(t *testing.T) {
	db := setupHoldingsTestDB(t)

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID.String()}, {Key: "id", Value: "nonexistent"}}
	c.Request.SetRequestURI("/api/portfolios/" + testPortfolioID.String() + "/holdings/nonexistent")
	c.Request.Header.SetMethod("DELETE")
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID:   testUserID,
		Username: "testuser",
		Role:     "user",
	})

	DeleteHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 404 {
		t.Errorf("expected 404, got %d", c.Response.StatusCode())
	}
}

func TestDeleteHolding_OtherUserCannotDelete(t *testing.T) {
	db := setupHoldingsTestDB(t)
	id := createTestHolding(t, db, 10, 100, 900)

	otherPortfolioID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	otherUserID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
	db.Create(&models.Portfolio{ID: otherPortfolioID, UserID: otherUserID, Name: "Other", IsDefault: true})

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: otherPortfolioID.String()}, {Key: "id", Value: id}}
	c.Request.SetRequestURI("/api/portfolios/" + otherPortfolioID.String() + "/holdings/" + id)
	c.Request.Header.SetMethod("DELETE")
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID:   otherUserID,
		Username: "other",
		Role:     "user",
	})

	DeleteHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 404 {
		t.Errorf("expected 404 (not found for other user), got %d", c.Response.StatusCode())
	}

	var count int64
	db.Model(&models.Holding{}).Where("id = ?", id).Count(&count)
	if count != 1 {
		t.Error("holding should still exist")
	}
}

func createOtherUserHoldingLot(t *testing.T, db *gorm.DB) (uuid.UUID, uuid.UUID) {
	t.Helper()

	otherUserID := uuid.New()
	otherPortfolioID := uuid.New()
	holdingID := uuid.New()
	lotID := uuid.New()

	if err := db.Create(&models.Portfolio{
		ID: otherPortfolioID, UserID: otherUserID, Name: "Other",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Holding{
		ID: holdingID, UserID: otherUserID, PortfolioID: otherPortfolioID,
		AssetId: "stocks", Symbol: "PRIVATE", Name: "Private Holding",
		Currency: "CNY", Shares: decimal.NewFromInt(10), Price: decimal.NewFromInt(100),
		Value: decimal.NewFromInt(1000), Cost: decimal.NewFromInt(900), CostPrice: decimal.NewFromInt(90),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.HoldingLot{
		ID: lotID, HoldingID: holdingID, Type: "buy", Date: time.Now(),
		Shares: decimal.NewFromInt(10), CostPrice: decimal.NewFromInt(90),
		Cost: decimal.NewFromInt(900), ValueAdded: decimal.NewFromInt(1000),
	}).Error; err != nil {
		t.Fatal(err)
	}

	return holdingID, lotID
}

func TestUpdateLot_OtherUserCannotUpdate(t *testing.T) {
	db := setupHoldingsTestDB(t)
	holdingID, lotID := createOtherUserHoldingLot(t, db)

	c := newUserCtx("PATCH", "/api/portfolios/"+testPortfolioID.String()+"/holdings/"+holdingID.String()+"/lots/"+lotID.String(), map[string]any{
		"cost": "800",
	})
	c.Params = append(c.Params,
		param.Param{Key: "hid", Value: holdingID.String()},
		param.Param{Key: "lid", Value: lotID.String()},
	)

	UpdateLot(db)(context.Background(), c)

	if c.Response.StatusCode() != 404 {
		t.Fatalf("expected 404 for another user's holding, got %d: %s", c.Response.StatusCode(), c.Response.Body())
	}
	var lot models.HoldingLot
	if err := db.First(&lot, "id = ?", lotID).Error; err != nil {
		t.Fatal(err)
	}
	if !lot.Cost.Equal(decimal.NewFromInt(900)) {
		t.Fatalf("other user's lot was modified: expected cost 900, got %s", lot.Cost)
	}
}

func TestDeleteLot_OtherUserCannotDelete(t *testing.T) {
	db := setupHoldingsTestDB(t)
	holdingID, lotID := createOtherUserHoldingLot(t, db)

	c := newUserCtx("DELETE", "/api/portfolios/"+testPortfolioID.String()+"/holdings/"+holdingID.String()+"/lots/"+lotID.String(), nil)
	c.Params = append(c.Params,
		param.Param{Key: "hid", Value: holdingID.String()},
		param.Param{Key: "lid", Value: lotID.String()},
	)

	DeleteLot(db)(context.Background(), c)

	if c.Response.StatusCode() != 404 {
		t.Fatalf("expected 404 for another user's holding, got %d: %s", c.Response.StatusCode(), c.Response.Body())
	}
	var count int64
	if err := db.Model(&models.HoldingLot{}).Where("id = ?", lotID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("other user's lot should still exist")
	}
}

func TestConvertHoldingsCurrency_SameCurrency_NoChange(t *testing.T) {
	holdings := []models.Holding{
		{Currency: "CNY", Value: decimal.NewFromInt(1000), Cost: decimal.NewFromInt(800), Price: decimal.NewFromInt(100), CostPrice: decimal.NewFromInt(80)},
	}
	lotsMap := make(map[uuid.UUID][]models.HoldingLot)
	if err := convertHoldingsCurrency(holdings, lotsMap, "CNY", testRouter(), testUserID); err != nil {
		t.Fatal(err)
	}
	if !holdings[0].Value.Equal(decimal.NewFromInt(1000)) {
		t.Errorf("expected value unchanged at 1000, got %s", holdings[0].Value)
	}
	if holdings[0].Currency != "CNY" {
		t.Errorf("expected currency unchanged at CNY, got %s", holdings[0].Currency)
	}
}

func TestConvertHoldingsCurrency_EmptyCurrency_NoChange(t *testing.T) {
	holdings := []models.Holding{
		{Currency: "", Value: decimal.NewFromInt(500), Cost: decimal.NewFromInt(400), Price: decimal.NewFromInt(50), CostPrice: decimal.NewFromInt(40)},
	}
	lotsMap := make(map[uuid.UUID][]models.HoldingLot)
	if err := convertHoldingsCurrency(holdings, lotsMap, "CNY", testRouter(), testUserID); err != nil {
		t.Fatal(err)
	}
	if !holdings[0].Value.Equal(decimal.NewFromInt(500)) {
		t.Errorf("expected value unchanged at 500, got %s", holdings[0].Value)
	}
}

func TestListHoldings_WithCurrencyParam(t *testing.T) {
	db := setupHoldingsTestDB(t)
	createTestHoldingWithCurrency(t, db, "USD", 10, 100, 900)

	c := newUserCtx("GET", "/api/holdings?currency=CNY", nil)
	ListHoldings(db, testRouter())(context.Background(), c)

	// Exchange rate fetch fails in test environment (no Yahoo API), expect 500
	if c.Response.StatusCode() != 500 {
		t.Fatalf("expected 500 (exchange rate unavailable), got %d", c.Response.StatusCode())
	}
}

func TestListHoldings_WithoutCurrencyParam_OriginalValues(t *testing.T) {
	db := setupHoldingsTestDB(t)
	createTestHoldingWithCurrency(t, db, "USD", 10, 100, 900)

	c := newUserCtx("GET", "/api/holdings", nil)
	ListHoldings(db, testRouter())(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", c.Response.StatusCode())
	}

	var holdings []models.Holding
	if err := json.Unmarshal(c.Response.Body(), &holdings); err != nil {
		t.Fatal(err)
	}
	if len(holdings) != 1 {
		t.Fatalf("expected 1 holding, got %d", len(holdings))
	}
	if holdings[0].Currency != "USD" {
		t.Errorf("expected original currency USD, got %s", holdings[0].Currency)
	}
	if !holdings[0].Price.Equal(decimal.NewFromInt(100)) {
		t.Errorf("expected original price 100, got %s", holdings[0].Price)
	}
}
