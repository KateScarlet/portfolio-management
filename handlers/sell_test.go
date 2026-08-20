package handlers

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
	"uuid"

	"portfolio-management/middleware"
	"portfolio-management/models"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route/param"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	testUserID      = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	testPortfolioID = uuid.MustParse("00000000-0000-0000-0000-000000000002")
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		dsn = "postgres://localhost:5432/portfolio_test?sslmode=disable"
	}
	adminDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	adminSQLDB, err := adminDB.DB()
	if err != nil {
		t.Fatal(err)
	}

	schema := "test_" + uuid.New().String()
	if err := adminDB.Exec(fmt.Sprintf(`CREATE SCHEMA %q`, schema)).Error; err != nil {
		adminSQLDB.Close() //nolint:errcheck // best effort after setup failure
		t.Fatal(err)
	}

	testURL, err := url.Parse(dsn)
	if err != nil {
		adminDB.Exec(fmt.Sprintf(`DROP SCHEMA %q CASCADE`, schema)) //nolint:errcheck // best effort after setup failure
		adminSQLDB.Close()                                          //nolint:errcheck // best effort after setup failure
		t.Fatal(err)
	}
	query := testURL.Query()
	query.Set("search_path", schema)
	testURL.RawQuery = query.Encode()

	db, err := gorm.Open(postgres.Open(testURL.String()), &gorm.Config{})
	if err != nil {
		adminDB.Exec(fmt.Sprintf(`DROP SCHEMA %q CASCADE`, schema)) //nolint:errcheck // best effort after setup failure
		adminSQLDB.Close()                                          //nolint:errcheck // best effort after setup failure
		t.Fatal(err)
	}
	testSQLDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		testSQLDB.Close() //nolint:errcheck // test cleanup
		adminDB.Exec(fmt.Sprintf(`DROP SCHEMA %q CASCADE`, schema))
		adminSQLDB.Close() //nolint:errcheck // test cleanup
	})

	if err := db.AutoMigrate(
		&models.Portfolio{}, &models.Account{}, &models.Holding{}, &models.HoldingLot{},
		&models.Setting{}, &models.PortfolioRecord{}, &models.AvailableFund{},
		&models.FundTransaction{}, &models.User{}, &models.WebAuthnCredential{},
		&models.WebAuthnSession{}, &models.Dividend{},
	); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_available_funds_unique ON available_funds(user_id, portfolio_id, currency)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Portfolio{
		ID:        testPortfolioID,
		UserID:    testUserID,
		Name:      "默认组合",
		IsDefault: true,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func createTestHolding(t *testing.T, db *gorm.DB, shares, price, cost float64) string {
	t.Helper()
	dShares := decimal.NewFromFloat(shares)
	dPrice := decimal.NewFromFloat(price)
	dCost := decimal.NewFromFloat(cost)
	id := uuid.New()
	h := models.Holding{
		ID:          id,
		UserID:      testUserID,
		PortfolioID: testPortfolioID,
		AssetId:     "stocks",
		Symbol:      "TEST",
		Name:        "Test Stock",
		Shares:      dShares,
		Price:       dPrice,
		Value:       dShares.Mul(dPrice),
		Cost:        dCost,
	}
	var defaultAccount models.Account
	if err := db.Where("user_id = ? AND is_default = ?", testUserID, true).Find(&defaultAccount).Error; err == nil && defaultAccount.ID != uuid.Nil() {
		h.AccountID = defaultAccount.ID
	}
	if shares > 0 {
		h.CostPrice = dCost.Div(dShares)
	}
	if err := db.Create(&h).Error; err != nil {
		t.Fatal(err)
	}

	lot := models.HoldingLot{
		ID:        uuid.New(),
		HoldingID: id,
		Date:      time.UnixMilli(1000000),
		Shares:    dShares,
		Cost:      dCost,
		Fee:       decimal.Zero,
	}
	if shares > 0 {
		lot.CostPrice = dCost.Div(dShares)
		lot.ValueAdded = dShares.Mul(dPrice)
	} else {
		lot.ValueAdded = dPrice
	}
	if err := db.Create(&lot).Error; err != nil {
		t.Fatal(err)
	}
	return id.String()
}

func newCtx(id string, body any) *app.RequestContext {
	c := app.NewContext(1)
	c.Params = param.Params{{Key: "pid", Value: testPortfolioID.String()}, {Key: "id", Value: id}}
	c.Request.SetRequestURI("/api/portfolios/" + testPortfolioID.String() + "/holdings/" + id + "/sell")
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

	c := newCtx(id, SellRequest{Shares: decimal.NewFromInt(5), Price: decimal.NewFromInt(100), Fee: decimal.NewFromInt(600)})
	SellHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 400 {
		t.Errorf("expected 400, got %d", c.Response.StatusCode())
	}
	var resp map[string]string
	if err := json.Unmarshal(c.Response.Body(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"] != "手续费不能超过卖出收入" {
		t.Errorf("unexpected error: %q", resp["error"])
	}
}

func TestSell_FeeExceedsProceeds_ValueBased(t *testing.T) {
	db := setupTestDB(t)
	id := createTestHolding(t, db, 0, 0, 400)
	db.Model(&models.Holding{}).Where("id = ?", id).Updates(map[string]any{
		"value": decimal.NewFromInt(500), "shares": decimal.Zero, "price": decimal.Zero,
	})

	c := newCtx(id, SellRequest{Value: decimal.NewFromInt(300), Fee: decimal.NewFromInt(300)})
	SellHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 400 {
		t.Errorf("expected 400, got %d", c.Response.StatusCode())
	}
}

func TestSell_FeeJustUnderProceeds_ShouldPass(t *testing.T) {
	db := setupTestDB(t)
	id := createTestHolding(t, db, 10, 100, 900)

	c := newCtx(id, SellRequest{Shares: decimal.NewFromInt(5), Price: decimal.NewFromInt(100), Fee: decimal.NewFromInt(499)})
	SellHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Errorf("expected 200, got %d", c.Response.StatusCode())
	}
}

func TestSell_FeeEqualsProceeds_ShouldFail(t *testing.T) {
	db := setupTestDB(t)
	id := createTestHolding(t, db, 10, 100, 900)

	c := newCtx(id, SellRequest{Shares: decimal.NewFromInt(5), Price: decimal.NewFromInt(100), Fee: decimal.NewFromInt(500)})
	SellHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 400 {
		t.Errorf("expected 400, got %d", c.Response.StatusCode())
	}
}

func TestSell_ZeroFee_ShouldPass(t *testing.T) {
	db := setupTestDB(t)
	id := createTestHolding(t, db, 10, 100, 900)

	c := newCtx(id, SellRequest{Shares: decimal.NewFromInt(5), Price: decimal.NewFromInt(100), Fee: decimal.Zero})
	SellHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Errorf("expected 200, got %d", c.Response.StatusCode())
	}
}

func TestSell_ProceedsGoToCorrectCurrencyFund(t *testing.T) {
	db := setupTestDB(t)
	id := createTestHoldingWithCurrency(t, db, "USD", 10, 100, 900)

	db.Create(&models.AvailableFund{
		ID: uuid.New(), UserID: testUserID, PortfolioID: testPortfolioID,
		Currency: "USD", Amount: decimal.NewFromInt(1000),
	})

	c := newCtx(id, SellRequest{Shares: decimal.NewFromInt(5), Price: decimal.NewFromInt(100), Fee: decimal.Zero})
	SellHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d: %s", c.Response.StatusCode(), string(c.Response.Body()))
	}

	var af models.AvailableFund
	db.Where("user_id = ? AND portfolio_id = ? AND currency = ?", testUserID, testPortfolioID, "USD").First(&af)
	expected := decimal.NewFromInt(1500) // 1000 + 5*100
	if !af.Amount.Equal(expected) {
		t.Errorf("expected USD funds %s, got %s", expected, af.Amount)
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

	c := newCtx(id, SellRequest{Shares: decimal.NewFromInt(5), Price: decimal.NewFromInt(100), Fee: decimal.NewFromInt(10)})
	SellHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", c.Response.StatusCode())
	}

	var af models.AvailableFund
	db.Where("user_id = ? AND portfolio_id = ? AND currency = ?", testUserID, testPortfolioID, "CNY").First(&af)
	expected := decimal.NewFromInt(490) // 5*100 - 10
	if !af.Amount.Equal(expected) {
		t.Errorf("expected CNY funds %s, got %s", expected, af.Amount)
	}
}

func TestSell_NewCurrencyFundCreatedOnSell(t *testing.T) {
	db := setupTestDB(t)
	id := createTestHoldingWithCurrency(t, db, "HKD", 10, 200, 1800)

	c := newCtx(id, SellRequest{Shares: decimal.NewFromInt(5), Price: decimal.NewFromInt(200), Fee: decimal.Zero})
	SellHolding(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", c.Response.StatusCode())
	}

	var af models.AvailableFund
	db.Where("user_id = ? AND portfolio_id = ? AND currency = ?", testUserID, testPortfolioID, "HKD").First(&af)
	if !af.Amount.Equal(decimal.NewFromInt(1000)) {
		t.Errorf("expected HKD funds 1000, got %s", af.Amount)
	}
}

func TestSell_ConcurrentRequestsCannotOversell(t *testing.T) {
	db := setupTestDB(t)
	id := createTestHoldingWithCurrency(t, db, "USD", 10, 100, 900)

	start := make(chan struct{})
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			<-start
			c := newCtx(id, SellRequest{
				Shares: decimal.NewFromInt(8),
				Price:  decimal.NewFromInt(100),
			})
			SellHolding(db)(context.Background(), c)
			statuses <- c.Response.StatusCode()
		})
	}
	close(start)
	wg.Wait()
	close(statuses)

	successes := 0
	rejections := 0
	for status := range statuses {
		switch status {
		case 200:
			successes++
		case 400:
			rejections++
		default:
			t.Fatalf("unexpected response status: %d", status)
		}
	}
	if successes != 1 || rejections != 1 {
		t.Fatalf("expected one successful sale and one rejection, got successes=%d rejections=%d", successes, rejections)
	}

	var holding models.Holding
	if err := db.First(&holding, "id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	if !holding.Shares.Equal(decimal.NewFromInt(2)) {
		t.Errorf("expected 2 remaining shares, got %s", holding.Shares)
	}

	var lots []models.HoldingLot
	if err := db.Where("holding_id = ?", id).Find(&lots).Error; err != nil {
		t.Fatal(err)
	}
	if len(lots) != 2 {
		t.Errorf("expected one buy lot and one sell lot, got %d lots", len(lots))
	}

	var fund models.AvailableFund
	if err := db.Where("user_id = ? AND portfolio_id = ? AND currency = ?", testUserID, testPortfolioID, "USD").First(&fund).Error; err != nil {
		t.Fatal(err)
	}
	if !fund.Amount.Equal(decimal.NewFromInt(800)) {
		t.Errorf("expected sale proceeds of 800, got %s", fund.Amount)
	}
}

func createTestHoldingWithCurrency(t *testing.T, db *gorm.DB, currency string, shares, price, cost float64) string {
	t.Helper()
	dShares := decimal.NewFromFloat(shares)
	dPrice := decimal.NewFromFloat(price)
	dCost := decimal.NewFromFloat(cost)
	id := uuid.New()
	h := models.Holding{
		ID:          id,
		UserID:      testUserID,
		PortfolioID: testPortfolioID,
		AssetId:     "stocks",
		Symbol:      "TEST",
		Name:        "Test Stock",
		Currency:    currency,
		Shares:      dShares,
		Price:       dPrice,
		Value:       dShares.Mul(dPrice),
		Cost:        dCost,
		CostPrice:   dCost.Div(dShares),
	}
	if err := db.Create(&h).Error; err != nil {
		t.Fatal(err)
	}

	lot := models.HoldingLot{
		ID:         uuid.New(),
		HoldingID:  id,
		Date:       time.UnixMilli(1000000),
		Shares:     dShares,
		CostPrice:  dCost.Div(dShares),
		Cost:       dCost,
		ValueAdded: dShares.Mul(dPrice),
		Fee:        decimal.Zero,
	}
	if err := db.Create(&lot).Error; err != nil {
		t.Fatal(err)
	}
	return id.String()
}
