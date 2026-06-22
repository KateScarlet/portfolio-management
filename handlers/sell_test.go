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

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Holding{}, &models.Setting{}); err != nil {
		t.Fatal(err)
	}
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
		ID:      id,
		UserID:  testUserID,
		AssetId: "stocks",
		Symbol:  "TEST",
		Name:    "Test Stock",
		Shares:  shares,
		Price:   price,
		Value:   shares * price,
		Cost:    cost,
		Lots:    lots,
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
	c.Params = param.Params{{Key: "id", Value: id}}
	c.Request.SetRequestURI("/api/holdings/" + id + "/sell")
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
