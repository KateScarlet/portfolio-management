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

func setupRecordsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return setupTestDB(t)
}

func createTestRecord(t *testing.T, db *gorm.DB, timestamp int64) string {
	t.Helper()
	id := uuid.New().String()
	r := models.PortfolioRecord{
		ID:          id,
		UserID:      testUserID,
		PortfolioID: testPortfolioID,
		Timestamp:   timestamp,
		Assets:      models.AssetMapColumn{"stocks": 1000, "bonds": 500},
		Total:       1500,
		Principal:   1400,
	}
	if err := db.Create(&r).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

// --- ListRecords ---

func TestListRecords_Empty(t *testing.T) {
	db := setupRecordsTestDB(t)
	c := newUserCtx("GET", "/api/records", nil)

	ListRecords(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", c.Response.StatusCode())
	}
	var records []models.PortfolioRecord
	json.Unmarshal(c.Response.Body(), &records)
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestListRecords_ReturnsUserRecords(t *testing.T) {
	db := setupRecordsTestDB(t)
	createTestRecord(t, db, 1000)
	createTestRecord(t, db, 2000)

	c := newUserCtx("GET", "/api/records", nil)
	ListRecords(db)(context.Background(), c)

	var records []models.PortfolioRecord
	json.Unmarshal(c.Response.Body(), &records)
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	// Should be ordered by timestamp DESC
	if records[0].Timestamp < records[1].Timestamp {
		t.Error("expected descending order by timestamp")
	}
}

func TestListRecords_Unauthorized(t *testing.T) {
	db := setupRecordsTestDB(t)
	c := app.NewContext(1)
	c.Request.SetRequestURI("/api/records")

	ListRecords(db)(context.Background(), c)

	if c.Response.StatusCode() != 401 {
		t.Errorf("expected 401, got %d", c.Response.StatusCode())
	}
}

// --- CreateRecord ---

func TestCreateRecord_FromHoldings(t *testing.T) {
	db := setupRecordsTestDB(t)
	// Need holdings for CreateRecord to work
	db.Create(&models.Holding{
		ID: uuid.New().String(), UserID: testUserID, PortfolioID: testPortfolioID, AssetId: "stocks",
		Symbol: "AAPL", Shares: 10, Price: 100, Value: 1000, Cost: 900,
		Lots: models.JSONColumn{{ID: uuid.New().String(), Shares: 10, Cost: 900, ValueAdded: 1000}},
	})

	c := newUserCtx("POST", "/api/records", nil)
	CreateRecord(db)(context.Background(), c)

	if c.Response.StatusCode() != 201 {
		t.Fatalf("expected 201, got %d: %s", c.Response.StatusCode(), string(c.Response.Body()))
	}

	var record models.PortfolioRecord
	json.Unmarshal(c.Response.Body(), &record)
	if record.Total != 1000 {
		t.Errorf("expected total 1000, got %f", record.Total)
	}
	if record.Assets["stocks"] != 1000 {
		t.Errorf("expected stocks 1000, got %f", record.Assets["stocks"])
	}
	if len(record.Holdings) != 1 {
		t.Fatalf("expected 1 holding snapshot, got %d", len(record.Holdings))
	}
	if record.Holdings[0].Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %s", record.Holdings[0].Symbol)
	}
	if record.Holdings[0].Value != 1000 {
		t.Errorf("expected holding value 1000, got %f", record.Holdings[0].Value)
	}
	if record.Holdings[0].Cost != 900 {
		t.Errorf("expected holding cost 900, got %f", record.Holdings[0].Cost)
	}
}

func TestCreateRecord_NoHoldings(t *testing.T) {
	db := setupRecordsTestDB(t)

	c := newUserCtx("POST", "/api/records", nil)
	CreateRecord(db)(context.Background(), c)

	if c.Response.StatusCode() != 400 {
		t.Errorf("expected 400, got %d", c.Response.StatusCode())
	}
}

// --- DeleteRecord ---

func TestDeleteRecord_Success(t *testing.T) {
	db := setupRecordsTestDB(t)
	id := createTestRecord(t, db, 1000)

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID}, {Key: "id", Value: id}}
	c.Request.SetRequestURI("/api/portfolios/" + testPortfolioID + "/records/" + id)
	c.Request.Header.SetMethod("DELETE")
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID: testUserID, Username: "testuser", Role: "user",
	})

	DeleteRecord(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", c.Response.StatusCode())
	}

	var count int64
	db.Model(&models.PortfolioRecord{}).Where("id = ?", id).Count(&count)
	if count != 0 {
		t.Error("expected record to be deleted")
	}
}

func TestDeleteRecord_NotFound(t *testing.T) {
	db := setupRecordsTestDB(t)

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID}, {Key: "id", Value: "nonexistent"}}
	c.Request.SetRequestURI("/api/portfolios/" + testPortfolioID + "/records/nonexistent")
	c.Request.Header.SetMethod("DELETE")
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID: testUserID, Username: "testuser", Role: "user",
	})

	DeleteRecord(db)(context.Background(), c)

	if c.Response.StatusCode() != 404 {
		t.Errorf("expected 404, got %d", c.Response.StatusCode())
	}
}

// --- Settings ---

func TestListSettings_Empty(t *testing.T) {
	db := setupTestDB(t)
	c := newUserCtx("GET", "/api/settings", nil)

	ListSettings(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", c.Response.StatusCode())
	}
	var result map[string]string
	json.Unmarshal(c.Response.Body(), &result)
	if len(result) != 0 {
		t.Errorf("expected empty settings, got %v", result)
	}
}

func TestListSettings_ReturnsUserSettings(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.Setting{Key: "syncInterval", Value: "5", UserID: testUserID, PortfolioID: testPortfolioID})
	db.Create(&models.Setting{Key: "driftThreshold", Value: "10", UserID: testUserID, PortfolioID: testPortfolioID})

	c := newUserCtx("GET", "/api/settings", nil)
	ListSettings(db)(context.Background(), c)

	var result map[string]string
	json.Unmarshal(c.Response.Body(), &result)
	if result["syncInterval"] != "5" {
		t.Errorf("expected syncInterval=5, got %s", result["syncInterval"])
	}
	if result["driftThreshold"] != "10" {
		t.Errorf("expected driftThreshold=10, got %s", result["driftThreshold"])
	}
}

func TestGetAvailableFunds_Default(t *testing.T) {
	db := setupTestDB(t)
	c := newUserCtx("GET", "/api/funds", nil)

	GetAvailableFunds(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", c.Response.StatusCode())
	}
	var result []map[string]any
	json.Unmarshal(c.Response.Body(), &result)
	if len(result) != 0 {
		t.Errorf("expected empty funds array, got %v", result)
	}
}

func TestGetAvailableFunds_WithValue(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.AvailableFund{
		ID:          uuid.New().String(),
		UserID:      testUserID,
		PortfolioID: testPortfolioID,
		Currency:    "CNY",
		Amount:      50000.50,
	})

	c := newUserCtx("GET", "/api/funds", nil)
	GetAvailableFunds(db)(context.Background(), c)

	var result []map[string]any
	json.Unmarshal(c.Response.Body(), &result)
	if len(result) != 1 {
		t.Fatalf("expected 1 fund entry, got %d", len(result))
	}
	if result[0]["currency"] != "CNY" {
		t.Errorf("expected currency 'CNY', got %q", result[0]["currency"])
	}
}

func TestGetAvailableFunds_MultipleCurrencies(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.AvailableFund{
		ID: uuid.New().String(), UserID: testUserID, PortfolioID: testPortfolioID,
		Currency: "CNY", Amount: 10000,
	})
	db.Create(&models.AvailableFund{
		ID: uuid.New().String(), UserID: testUserID, PortfolioID: testPortfolioID,
		Currency: "USD", Amount: 5000,
	})

	c := newUserCtx("GET", "/api/funds", nil)
	GetAvailableFunds(db)(context.Background(), c)

	var result []map[string]any
	json.Unmarshal(c.Response.Body(), &result)
	if len(result) != 2 {
		t.Fatalf("expected 2 fund entries, got %d", len(result))
	}
}

func TestGetAvailableFunds_HidesZeroAmount(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.AvailableFund{
		ID: uuid.New().String(), UserID: testUserID, PortfolioID: testPortfolioID,
		Currency: "CNY", Amount: 0,
	})
	db.Create(&models.AvailableFund{
		ID: uuid.New().String(), UserID: testUserID, PortfolioID: testPortfolioID,
		Currency: "USD", Amount: 100,
	})

	c := newUserCtx("GET", "/api/funds", nil)
	GetAvailableFunds(db)(context.Background(), c)

	var result []map[string]any
	json.Unmarshal(c.Response.Body(), &result)
	if len(result) != 1 {
		t.Fatalf("expected 1 fund entry (zero hidden), got %d", len(result))
	}
	if result[0]["currency"] != "USD" {
		t.Errorf("expected USD, got %q", result[0]["currency"])
	}
}

func TestTransferIn_Success(t *testing.T) {
	db := setupTestDB(t)

	c := newUserCtx("POST", "/api/funds/transfer-in", map[string]any{"currency": "USD", "amount": 3000, "note": "test"})
	TransferIn(db)(context.Background(), c)

	if c.Response.StatusCode() != 201 {
		t.Fatalf("expected 201, got %d: %s", c.Response.StatusCode(), string(c.Response.Body()))
	}

	var af models.AvailableFund
	db.Where("user_id = ? AND portfolio_id = ? AND currency = ?", testUserID, testPortfolioID, "USD").First(&af)
	if af.Amount != 3000 {
		t.Errorf("expected 3000, got %.2f", af.Amount)
	}

	var tx models.FundTransaction
	db.Where("portfolio_id = ? AND type = ?", testPortfolioID, "transfer_in").First(&tx)
	if tx.Amount != 3000 || tx.Currency != "USD" {
		t.Errorf("expected transaction 3000 USD, got %.2f %s", tx.Amount, tx.Currency)
	}
}

func TestTransferOut_InsufficientFunds(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.AvailableFund{
		ID: uuid.New().String(), UserID: testUserID, PortfolioID: testPortfolioID,
		Currency: "HKD", Amount: 100,
	})

	c := newUserCtx("POST", "/api/funds/transfer-out", map[string]any{"currency": "HKD", "amount": 200})
	TransferOut(db)(context.Background(), c)

	if c.Response.StatusCode() != 400 {
		t.Errorf("expected 400 for insufficient funds, got %d", c.Response.StatusCode())
	}
}

func TestTransferOut_Success(t *testing.T) {
	db := setupTestDB(t)
	db.Create(&models.AvailableFund{
		ID: uuid.New().String(), UserID: testUserID, PortfolioID: testPortfolioID,
		Currency: "HKD", Amount: 100,
	})

	c := newUserCtx("POST", "/api/funds/transfer-out", map[string]any{"currency": "HKD", "amount": 50, "note": "withdraw"})
	TransferOut(db)(context.Background(), c)

	if c.Response.StatusCode() != 201 {
		t.Fatalf("expected 201, got %d: %s", c.Response.StatusCode(), string(c.Response.Body()))
	}

	var af models.AvailableFund
	db.Where("user_id = ? AND portfolio_id = ? AND currency = ?", testUserID, testPortfolioID, "HKD").First(&af)
	if af.Amount != 50 {
		t.Errorf("expected 50, got %.2f", af.Amount)
	}
}

func TestTransferIn_MissingCurrency(t *testing.T) {
	db := setupTestDB(t)

	c := newUserCtx("POST", "/api/funds/transfer-in", map[string]any{"amount": 100})
	TransferIn(db)(context.Background(), c)

	if c.Response.StatusCode() != 400 {
		t.Errorf("expected 400 for missing currency, got %d", c.Response.StatusCode())
	}
}
