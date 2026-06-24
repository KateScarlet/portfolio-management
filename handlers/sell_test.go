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
	"github.com/libtnb/sqlite"
	"gorm.io/gorm"
)

const testUserID = "test-user-id"
const testPortfolioID = "test-portfolio-id"

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Portfolio{}, &models.Holding{}, &models.Setting{}, &models.PortfolioRecord{}, &models.AvailableFund{}); err != nil {
		t.Fatal(err)
	}
	db.Create(&models.Portfolio{
		ID:        testPortfolioID,
		UserID:    testUserID,
		Name:      "默认组合",
		IsDefault: true,
		CreatedAt: 1000,
	})
	return db
}

func createTestHolding(t *testing.T, db *gorm.DB, shares, price, cost float64) string {
	t.Helper()
	id := uuid.New().String()
	var lots models.JSONColumn
	if shares > 0 {
		lots = models.JSONColumn{
			{
				ID:         uuid.New().String(),
				Date:       1000000,
				Shares:     shares,
				CostPrice:  cost / shares,
				Cost:       cost,
				ValueAdded: shares * price,
				Fee:        0,
			},
		}
	} else {
		lots = models.JSONColumn{
			{
				ID:         uuid.New().String(),
				Date:       1000000,
				Shares:     0,
				Cost:       cost,
				ValueAdded: price,
				Fee:        0,
			},
		}
	}
	h := models.Holding{
		ID:          id,
		UserID:      testUserID,
		PortfolioID: testPortfolioID,
		AssetId:     "stocks",
		Symbol:      "TEST",
		Name:        "Test Stock",
		Shares:      shares,
		Price:       price,
		Value:       shares * price,
		Cost:        cost,
		Lots:        lots,
	}
	if shares > 0 {
		h.CostPrice = cost / shares
	}
	if err := db.Create(&h).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func newCtx(id string, body any) *app.RequestContext {
	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID}, {Key: "id", Value: id}}
	c.Request.SetRequestURI("/api/portfolios/" + testPortfolioID + "/holdings/" + id + "/sell")
	c.Request.Header.SetMethod("POST")
	c.Request.Header.SetContentTypeBytes([]byte("application/json"))
	b, _ := json.Marshal(body)
	c.Request.SetBodyRaw(b)
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID:   testUserID,
		Username: "testuser",
		Role:     "user",
	})
	return c
}

func TestSell_FeeExceedsProceeds_ShareBased(t *testing.T) {
	db := setupTestDB(t)
	id := createTestHolding(t, db, 10, 100, 900)

	c := newCtx(id, SellRequest{Shares: 5, Price: 100, Fee: 600})
	SellHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 400 {
		t.Errorf("expected 400, got %d", c.Response.StatusCode())
	}
	var resp map[string]string
	if err := json.Unmarshal(c.Response.Body(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"] != "Fee cannot exceed sell proceeds" {
		t.Errorf("unexpected error: %q", resp["error"])
	}
}

func TestSell_FeeExceedsProceeds_ValueBased(t *testing.T) {
	db := setupTestDB(t)
	id := createTestHolding(t, db, 0, 0, 400)
	db.Model(&models.Holding{}).Where("id = ?", id).Updates(map[string]any{
		"value": 500, "shares": 0, "price": 0,
	})

	c := newCtx(id, SellRequest{Value: 300, Fee: 300})
	SellHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 400 {
		t.Errorf("expected 400, got %d", c.Response.StatusCode())
	}
}

func TestSell_FeeJustUnderProceeds_ShouldPass(t *testing.T) {
	db := setupTestDB(t)
	id := createTestHolding(t, db, 10, 100, 900)

	c := newCtx(id, SellRequest{Shares: 5, Price: 100, Fee: 499})
	SellHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Errorf("expected 200, got %d", c.Response.StatusCode())
	}
}

func TestSell_FeeEqualsProceeds_ShouldFail(t *testing.T) {
	db := setupTestDB(t)
	id := createTestHolding(t, db, 10, 100, 900)

	c := newCtx(id, SellRequest{Shares: 5, Price: 100, Fee: 500})
	SellHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 400 {
		t.Errorf("expected 400, got %d", c.Response.StatusCode())
	}
}

func TestSell_ZeroFee_ShouldPass(t *testing.T) {
	db := setupTestDB(t)
	id := createTestHolding(t, db, 10, 100, 900)

	c := newCtx(id, SellRequest{Shares: 5, Price: 100, Fee: 0})
	SellHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Errorf("expected 200, got %d", c.Response.StatusCode())
	}
}

func TestSell_ProceedsGoToCorrectCurrencyFund(t *testing.T) {
	db := setupTestDB(t)
	id := createTestHoldingWithCurrency(t, db, "USD", 10, 100, 900)

	db.Create(&models.AvailableFund{
		ID: uuid.New().String(), UserID: testUserID, PortfolioID: testPortfolioID,
		Currency: "USD", Amount: 1000,
	})

	c := newCtx(id, SellRequest{Shares: 5, Price: 100, Fee: 0})
	SellHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d: %s", c.Response.StatusCode(), string(c.Response.Body()))
	}

	var af models.AvailableFund
	db.Where("user_id = ? AND portfolio_id = ? AND currency = ?", testUserID, testPortfolioID, "USD").First(&af)
	expected := 1500.0 // 1000 + 5*100
	if af.Amount != expected {
		t.Errorf("expected USD funds %.2f, got %.2f", expected, af.Amount)
	}

	var cnCount int64
	db.Model(&models.AvailableFund{}).Where("user_id = ? AND portfolio_id = ? AND currency = ?", testUserID, testPortfolioID, "CNY").Count(&cnCount)
	if cnCount != 0 {
		t.Errorf("expected no CNY fund created, but found %d", cnCount)
	}
}

func TestSell_ProceedsGoToCNYFundByDefault(t *testing.T) {
	db := setupTestDB(t)
	id := createTestHoldingWithCurrency(t, db, "CNY", 10, 100, 900)

	c := newCtx(id, SellRequest{Shares: 5, Price: 100, Fee: 10})
	SellHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", c.Response.StatusCode())
	}

	var af models.AvailableFund
	db.Where("user_id = ? AND portfolio_id = ? AND currency = ?", testUserID, testPortfolioID, "CNY").First(&af)
	expected := 490.0 // 5*100 - 10
	if af.Amount != expected {
		t.Errorf("expected CNY funds %.2f, got %.2f", expected, af.Amount)
	}
}

func TestSell_NewCurrencyFundCreatedOnSell(t *testing.T) {
	db := setupTestDB(t)
	id := createTestHoldingWithCurrency(t, db, "HKD", 10, 200, 1800)

	c := newCtx(id, SellRequest{Shares: 5, Price: 200, Fee: 0})
	SellHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", c.Response.StatusCode())
	}

	var af models.AvailableFund
	db.Where("user_id = ? AND portfolio_id = ? AND currency = ?", testUserID, testPortfolioID, "HKD").First(&af)
	if af.Amount != 1000 {
		t.Errorf("expected HKD funds 1000, got %.2f", af.Amount)
	}
}

func createTestHoldingWithCurrency(t *testing.T, db *gorm.DB, currency string, shares, price, cost float64) string {
	t.Helper()
	id := uuid.New().String()
	lots := models.JSONColumn{
		{
			ID:         uuid.New().String(),
			Date:       1000000,
			Shares:     shares,
			CostPrice:  cost / shares,
			Cost:       cost,
			ValueAdded: shares * price,
			Fee:        0,
		},
	}
	h := models.Holding{
		ID:          id,
		UserID:      testUserID,
		PortfolioID: testPortfolioID,
		AssetId:     "stocks",
		Symbol:      "TEST",
		Name:        "Test Stock",
		Currency:    currency,
		Shares:      shares,
		Price:       price,
		Value:       shares * price,
		Cost:        cost,
		CostPrice:   cost / shares,
		Lots:        lots,
	}
	if err := db.Create(&h).Error; err != nil {
		t.Fatal(err)
	}
	return id
}
