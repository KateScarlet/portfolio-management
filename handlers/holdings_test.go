package handlers

import (
	"context"
	"encoding/json"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route/param"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func setupHoldingsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return setupTestDB(t)
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
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID}}
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

	ListHoldings(db)(context.Background(), c)

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
	ListHoldings(db)(context.Background(), c)

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

	otherPortfolioID := "other-portfolio-id"
	db.Create(&models.Portfolio{ID: otherPortfolioID, UserID: "other-user", Name: "Other", IsDefault: true, CreatedAt: 1000})

	c := app.NewContext(1)
	c.Request.SetRequestURI("/api/portfolios/" + otherPortfolioID + "/holdings")
	c.Request.Header.SetMethod("GET")
	c.Params = param.Params{{Key: "pid", Value: otherPortfolioID}}
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID:   "other-user",
		Username: "other",
		Role:     "user",
	})

	ListHoldings(db)(context.Background(), c)

	var holdings []models.Holding
	json.Unmarshal(c.Response.Body(), &holdings)
	if len(holdings) != 0 {
		t.Errorf("expected 0 holdings for other user, got %d", len(holdings))
	}
}

func TestListHoldings_Unauthorized(t *testing.T) {
	db := setupHoldingsTestDB(t)
	c := app.NewContext(1)
	c.Request.SetRequestURI("/api/holdings")
	c.Request.Header.SetMethod("GET")

	ListHoldings(db)(context.Background(), c)

	if c.Response.StatusCode() != 401 {
		t.Errorf("expected 401, got %d", c.Response.StatusCode())
	}
}

// --- CreateHolding ---

func TestCreateHolding_NewStockHolding(t *testing.T) {
	db := setupHoldingsTestDB(t)
	body := map[string]any{
		"assetId":    "stocks",
		"symbol":     "AAPL",
		"name":       "Apple",
		"shares":     10,
		"price":      150,
		"costPrice":  140,
		"cost":       1400,
		"value":      1500,
		"deductFromCash": false,
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
	if holding.Shares != 10 {
		t.Errorf("expected shares 10, got %f", holding.Shares)
	}
}

func TestCreateHolding_MergesIntoExisting(t *testing.T) {
	db := setupHoldingsTestDB(t)
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
	var holding models.Holding
	json.Unmarshal(c.Response.Body(), &holding)
	if holding.Shares != 15 {
		t.Errorf("expected merged shares 15, got %f", holding.Shares)
	}
	if len(holding.Lots) != 2 {
		t.Errorf("expected 2 lots, got %d", len(holding.Lots))
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
	db.Create(&models.Setting{Key: "availableFunds", Value: "10000", UserID: testUserID, PortfolioID: testPortfolioID})

	body := map[string]any{
		"assetId":        "stocks",
		"symbol":         "VTI",
		"shares":         10,
		"price":          100,
		"costPrice":      100,
		"cost":           1000,
		"value":          1000,
		"fee":            5,
		"deductFromCash": true,
	}
	c := newUserCtx("POST", "/api/holdings", body)
	CreateHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 201 {
		t.Fatalf("expected 201, got %d: %s", c.Response.StatusCode(), string(c.Response.Body()))
	}

	var setting models.Setting
	db.Where("`key` = ? AND user_id = ?", "availableFunds", testUserID).First(&setting)
	expected := "8995.00" // 10000 - 1000 - 5
	if setting.Value != expected {
		t.Errorf("expected funds %s, got %s", expected, setting.Value)
	}
}

func TestCreateHolding_DeductFromCash_InsufficientFunds(t *testing.T) {
	db := setupHoldingsTestDB(t)
	db.Create(&models.Setting{Key: "availableFunds", Value: "500", UserID: testUserID, PortfolioID: testPortfolioID})

	body := map[string]any{
		"assetId":        "stocks",
		"symbol":         "VTI",
		"shares":         10,
		"price":          100,
		"costPrice":      100,
		"cost":           1000,
		"value":          1000,
		"deductFromCash": true,
	}
	c := newUserCtx("POST", "/api/holdings", body)
	CreateHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 500 {
		t.Errorf("expected 500 (insufficient funds), got %d", c.Response.StatusCode())
	}
}

// --- UpdateHolding ---

func TestUpdateHolding_ManualValueUpdate(t *testing.T) {
	db := setupHoldingsTestDB(t)
	id := uuid.New().String()
	lots := models.JSONColumn{
		{ID: uuid.New().String(), Date: 1000, Cost: 5000, ValueAdded: 5000},
	}
	h := models.Holding{
		ID:          id,
		UserID:      testUserID,
		PortfolioID: testPortfolioID,
		AssetId:     "bonds",
		Name:    "手工债券",
		Value:   5000,
		Cost:    5000,
		Lots:    lots,
	}
	db.Create(&h)

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID}, {Key: "id", Value: id}}
	c.Request.SetRequestURI("/api/portfolios/"+testPortfolioID+"/holdings/" + id)
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

	var updated models.Holding
	json.Unmarshal(c.Response.Body(), &updated)
	if updated.Value != 6000 {
		t.Errorf("expected value 6000, got %f (json.Number fix)", updated.Value)
	}
}

func TestUpdateHolding_BlockAssetIdChange(t *testing.T) {
	db := setupHoldingsTestDB(t)
	id := createTestHolding(t, db, 10, 100, 900)

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID}, {Key: "id", Value: id}}
	c.Request.SetRequestURI("/api/portfolios/"+testPortfolioID+"/holdings/" + id)
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
		{"id": uuid.New().String(), "date": 1000, "shares": 20, "costPrice": 50, "cost": 1000, "valueAdded": 2000},
	}
	body := map[string]any{"lots": newLots}

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID}, {Key: "id", Value: id}}
	c.Request.SetRequestURI("/api/portfolios/"+testPortfolioID+"/holdings/" + id)
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

	var updated models.Holding
	json.Unmarshal(c.Response.Body(), &updated)
	if updated.Shares != 20 {
		t.Errorf("expected shares 20, got %f", updated.Shares)
	}
	if updated.Cost != 1000 {
		t.Errorf("expected cost 1000, got %f", updated.Cost)
	}
}

func TestUpdateHolding_NotFound(t *testing.T) {
	db := setupHoldingsTestDB(t)

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID}, {Key: "id", Value: "nonexistent"}}
	c.Request.SetRequestURI("/api/portfolios/"+testPortfolioID+"/holdings/nonexistent")
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
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID}, {Key: "id", Value: id}}
	c.Request.SetRequestURI("/api/portfolios/"+testPortfolioID+"/holdings/" + id)
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
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID}, {Key: "id", Value: "nonexistent"}}
	c.Request.SetRequestURI("/api/portfolios/"+testPortfolioID+"/holdings/nonexistent")
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

	otherPortfolioID := "other-portfolio-id"
	db.Create(&models.Portfolio{ID: otherPortfolioID, UserID: "other-user", Name: "Other", IsDefault: true, CreatedAt: 1000})

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: otherPortfolioID}, {Key: "id", Value: id}}
	c.Request.SetRequestURI("/api/portfolios/"+otherPortfolioID+"/holdings/" + id)
	c.Request.Header.SetMethod("DELETE")
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID:   "other-user",
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
