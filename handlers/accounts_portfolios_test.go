package handlers

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"testing"
	"time"
	"uuid"

	"portfolio-management/models"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route/param"
	"github.com/shopspring/decimal"
)

func newResourceCtx(method, path string, body any) *app.RequestContext {
	c := newUserCtx(method, path, nil)
	if body != nil {
		payload, _ := json.Marshal(body)
		c.Request.SetBodyStream(bytes.NewReader(payload), len(payload))
	}
	return c
}

func TestAccounts_CRUDAndOwnership(t *testing.T) {
	db := setupTestDB(t)
	otherUserID := uuid.New()
	defaultAccount := models.Account{
		ID: uuid.New(), UserID: testUserID, Name: "Default", IsDefault: true,
	}
	otherAccount := models.Account{
		ID: uuid.New(), UserID: otherUserID, Name: "Other user's account",
	}
	if err := db.Create(&[]models.Account{defaultAccount, otherAccount}).Error; err != nil {
		t.Fatal(err)
	}

	createCtx := newResourceCtx("POST", "/api/accounts", map[string]any{
		"name": "Brokerage", "description": "Long-term", "broker": "Demo Broker",
	})
	CreateAccount(db)(context.Background(), createCtx)
	if createCtx.Response.StatusCode() != 201 {
		t.Fatalf("create: expected 201, got %d: %s", createCtx.Response.StatusCode(), createCtx.Response.Body())
	}
	var created models.Account
	if err := json.Unmarshal(createCtx.Response.Body(), &created); err != nil {
		t.Fatal(err)
	}
	if created.UserID != testUserID || created.Name != "Brokerage" || created.Broker != "Demo Broker" {
		t.Fatalf("unexpected created account: %+v", created)
	}

	listCtx := newResourceCtx("GET", "/api/accounts", nil)
	ListAccounts(db)(context.Background(), listCtx)
	if listCtx.Response.StatusCode() != 200 {
		t.Fatalf("list: expected 200, got %d", listCtx.Response.StatusCode())
	}
	var accounts []models.Account
	if err := json.Unmarshal(listCtx.Response.Body(), &accounts); err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 || accounts[0].ID != defaultAccount.ID {
		t.Fatalf("expected default and created accounts only, got %+v", accounts)
	}

	updateCtx := newResourceCtx("PATCH", "/api/accounts/"+created.ID.String(), map[string]any{
		"name": "Updated", "broker": "New Broker",
	})
	updateCtx.Params = param.Params{{Key: "id", Value: created.ID.String()}}
	UpdateAccount(db)(context.Background(), updateCtx)
	if updateCtx.Response.StatusCode() != 200 {
		t.Fatalf("update: expected 200, got %d: %s", updateCtx.Response.StatusCode(), updateCtx.Response.Body())
	}
	var updated models.Account
	if err := json.Unmarshal(updateCtx.Response.Body(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Updated" || updated.Broker != "New Broker" || updated.Description != "Long-term" {
		t.Fatalf("unexpected updated account: %+v", updated)
	}

	otherCtx := newResourceCtx("PATCH", "/api/accounts/"+otherAccount.ID.String(), map[string]any{"name": "stolen"})
	otherCtx.Params = param.Params{{Key: "id", Value: otherAccount.ID.String()}}
	UpdateAccount(db)(context.Background(), otherCtx)
	if otherCtx.Response.StatusCode() != 404 {
		t.Fatalf("cross-user update: expected 404, got %d", otherCtx.Response.StatusCode())
	}
}

func TestAccounts_ValidationAndAuthorization(t *testing.T) {
	db := setupTestDB(t)

	emptyNameCtx := newResourceCtx("POST", "/api/accounts", map[string]any{"name": ""})
	CreateAccount(db)(context.Background(), emptyNameCtx)
	if emptyNameCtx.Response.StatusCode() != 400 {
		t.Fatalf("empty name: expected 400, got %d", emptyNameCtx.Response.StatusCode())
	}

	account := models.Account{ID: uuid.New(), UserID: testUserID, Name: "Account"}
	if err := db.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	emptyUpdateCtx := newResourceCtx("PATCH", "/api/accounts/"+account.ID.String(), map[string]any{})
	emptyUpdateCtx.Params = param.Params{{Key: "id", Value: account.ID.String()}}
	UpdateAccount(db)(context.Background(), emptyUpdateCtx)
	if emptyUpdateCtx.Response.StatusCode() != 400 {
		t.Fatalf("empty update: expected 400, got %d", emptyUpdateCtx.Response.StatusCode())
	}

	unauthorizedCtx := newResourceCtx("GET", "/api/accounts", nil)
	unauthorizedCtx.ResetWithoutConn()
	ListAccounts(db)(context.Background(), unauthorizedCtx)
	if unauthorizedCtx.Response.StatusCode() != 401 {
		t.Fatalf("unauthorized list: expected 401, got %d", unauthorizedCtx.Response.StatusCode())
	}
}

func TestDeleteAccount_ReassignsHoldingsAndProtectsDefault(t *testing.T) {
	db := setupTestDB(t)
	defaultAccount := models.Account{ID: uuid.New(), UserID: testUserID, Name: "Default", IsDefault: true}
	secondary := models.Account{ID: uuid.New(), UserID: testUserID, Name: "Secondary"}
	if err := db.Create(&[]models.Account{defaultAccount, secondary}).Error; err != nil {
		t.Fatal(err)
	}
	holding := models.Holding{
		ID: uuid.New(), UserID: testUserID, PortfolioID: testPortfolioID,
		AccountID: secondary.ID, AssetId: "stocks", Symbol: "MOVE",
	}
	if err := db.Create(&holding).Error; err != nil {
		t.Fatal(err)
	}

	deleteCtx := newResourceCtx("DELETE", "/api/accounts/"+secondary.ID.String(), nil)
	deleteCtx.Params = param.Params{{Key: "id", Value: secondary.ID.String()}}
	DeleteAccount(db)(context.Background(), deleteCtx)
	if deleteCtx.Response.StatusCode() != 200 {
		t.Fatalf("delete: expected 200, got %d: %s", deleteCtx.Response.StatusCode(), deleteCtx.Response.Body())
	}
	if err := db.First(&holding, "id = ?", holding.ID).Error; err != nil {
		t.Fatal(err)
	}
	if holding.AccountID != defaultAccount.ID {
		t.Fatalf("expected holding reassigned to %s, got %s", defaultAccount.ID, holding.AccountID)
	}
	var count int64
	db.Model(&models.Account{}).Where("id = ?", secondary.ID).Count(&count)
	if count != 0 {
		t.Fatalf("expected secondary account deleted, count=%d", count)
	}

	defaultCtx := newResourceCtx("DELETE", "/api/accounts/"+defaultAccount.ID.String(), nil)
	defaultCtx.Params = param.Params{{Key: "id", Value: defaultAccount.ID.String()}}
	DeleteAccount(db)(context.Background(), defaultCtx)
	if defaultCtx.Response.StatusCode() != 400 {
		t.Fatalf("delete default: expected 400, got %d", defaultCtx.Response.StatusCode())
	}
}

func TestListAllAccountHoldings_FiltersAndAttachesAccountData(t *testing.T) {
	db := setupTestDB(t)
	firstAccount := models.Account{ID: uuid.New(), UserID: testUserID, Name: "Broker A", IsDefault: true}
	secondAccount := models.Account{ID: uuid.New(), UserID: testUserID, Name: "Broker B"}
	otherUserID := uuid.New()
	otherAccount := models.Account{ID: uuid.New(), UserID: otherUserID, Name: "Other Broker"}
	if err := db.Create(&[]models.Account{firstAccount, secondAccount, otherAccount}).Error; err != nil {
		t.Fatal(err)
	}
	firstHolding := models.Holding{
		ID: uuid.New(), UserID: testUserID, PortfolioID: testPortfolioID,
		AccountID: firstAccount.ID, AssetId: "stocks", Symbol: "AAA",
	}
	secondHolding := models.Holding{
		ID: uuid.New(), UserID: testUserID, PortfolioID: testPortfolioID,
		AccountID: secondAccount.ID, AssetId: "funds", Symbol: "BBB",
	}
	otherHolding := models.Holding{
		ID: uuid.New(), UserID: otherUserID, PortfolioID: uuid.New(),
		AccountID: otherAccount.ID, AssetId: "stocks", Symbol: "PRIVATE",
	}
	if err := db.Create(&[]models.Holding{firstHolding, secondHolding, otherHolding}).Error; err != nil {
		t.Fatal(err)
	}
	lot := models.HoldingLot{
		ID: uuid.New(), HoldingID: secondHolding.ID,
		Shares: decimal.NewFromInt(2), Cost: decimal.NewFromInt(100),
	}
	if err := db.Create(&lot).Error; err != nil {
		t.Fatal(err)
	}

	allCtx := newResourceCtx("GET", "/api/account-holdings", nil)
	ListAllAccountHoldings(db, testRouter())(context.Background(), allCtx)
	if allCtx.Response.StatusCode() != 200 {
		t.Fatalf("list all: expected 200, got %d: %s", allCtx.Response.StatusCode(), allCtx.Response.Body())
	}
	var all []HoldingWithAccount
	if err := json.Unmarshal(allCtx.Response.Body(), &all); err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected two own holdings, got %+v", all)
	}
	for _, holding := range all {
		if holding.Symbol == "PRIVATE" || holding.AccountName == "" {
			t.Fatalf("unexpected ownership/account enrichment result: %+v", all)
		}
	}

	filterCtx := newResourceCtx("GET", "/api/account-holdings?account_id="+secondAccount.ID.String(), nil)
	ListAllAccountHoldings(db, testRouter())(context.Background(), filterCtx)
	var filtered []HoldingWithAccount
	if err := json.Unmarshal(filterCtx.Response.Body(), &filtered); err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].ID != secondHolding.ID || filtered[0].AccountName != "Broker B" || len(filtered[0].Lots) != 1 {
		t.Fatalf("unexpected filtered holdings: %+v", filtered)
	}
}

func TestListAccountHoldings_OwnershipAndLots(t *testing.T) {
	db := setupTestDB(t)
	account := models.Account{ID: uuid.New(), UserID: testUserID, Name: "Broker"}
	otherAccount := models.Account{ID: uuid.New(), UserID: uuid.New(), Name: "Other"}
	if err := db.Create(&[]models.Account{account, otherAccount}).Error; err != nil {
		t.Fatal(err)
	}
	holding := models.Holding{
		ID: uuid.New(), UserID: testUserID, PortfolioID: testPortfolioID,
		AccountID: account.ID, AssetId: "stocks", Symbol: "LOTS",
	}
	if err := db.Create(&holding).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.HoldingLot{ID: uuid.New(), HoldingID: holding.ID, Shares: decimal.NewFromInt(3)}).Error; err != nil {
		t.Fatal(err)
	}

	c := newResourceCtx("GET", "/api/accounts/"+account.ID.String()+"/holdings", nil)
	c.Params = param.Params{{Key: "id", Value: account.ID.String()}}
	ListAccountHoldings(db, testRouter())(context.Background(), c)
	if c.Response.StatusCode() != 200 {
		t.Fatalf("own account: expected 200, got %d: %s", c.Response.StatusCode(), c.Response.Body())
	}
	var result []struct {
		models.Holding
		Lots []models.HoldingLot `json:"lots"`
	}
	if err := json.Unmarshal(c.Response.Body(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].ID != holding.ID || len(result[0].Lots) != 1 {
		t.Fatalf("unexpected account holdings: %+v", result)
	}

	otherCtx := newResourceCtx("GET", "/api/accounts/"+otherAccount.ID.String()+"/holdings", nil)
	otherCtx.Params = param.Params{{Key: "id", Value: otherAccount.ID.String()}}
	ListAccountHoldings(db, testRouter())(context.Background(), otherCtx)
	if otherCtx.Response.StatusCode() != 404 {
		t.Fatalf("other user's account: expected 404, got %d", otherCtx.Response.StatusCode())
	}
}

func TestPortfolios_CRUDAndOwnership(t *testing.T) {
	db := setupTestDB(t)
	other := models.Portfolio{ID: uuid.New(), UserID: uuid.New(), Name: "Other"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}

	createCtx := newResourceCtx("POST", "/api/portfolios", map[string]any{
		"name": "Retirement", "description": "Long-term plan",
	})
	CreatePortfolio(db)(context.Background(), createCtx)
	if createCtx.Response.StatusCode() != 201 {
		t.Fatalf("create: expected 201, got %d: %s", createCtx.Response.StatusCode(), createCtx.Response.Body())
	}
	var created models.Portfolio
	if err := json.Unmarshal(createCtx.Response.Body(), &created); err != nil {
		t.Fatal(err)
	}
	if created.UserID != testUserID || created.IsDefault || created.Name != "Retirement" {
		t.Fatalf("unexpected created portfolio: %+v", created)
	}

	listCtx := newResourceCtx("GET", "/api/portfolios", nil)
	ListPortfolios(db)(context.Background(), listCtx)
	var portfolios []models.Portfolio
	if err := json.Unmarshal(listCtx.Response.Body(), &portfolios); err != nil {
		t.Fatal(err)
	}
	if listCtx.Response.StatusCode() != 200 || len(portfolios) != 2 || portfolios[0].ID != testPortfolioID {
		t.Fatalf("expected default and created portfolios only, got %+v", portfolios)
	}

	updateCtx := newResourceCtx("PATCH", "/api/portfolios/"+created.ID.String(), map[string]any{
		"name": "Updated retirement",
	})
	updateCtx.Params = param.Params{{Key: "id", Value: created.ID.String()}}
	UpdatePortfolio(db)(context.Background(), updateCtx)
	if updateCtx.Response.StatusCode() != 200 {
		t.Fatalf("update: expected 200, got %d: %s", updateCtx.Response.StatusCode(), updateCtx.Response.Body())
	}
	var updated models.Portfolio
	if err := json.Unmarshal(updateCtx.Response.Body(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Updated retirement" || updated.Description != "Long-term plan" {
		t.Fatalf("unexpected updated portfolio: %+v", updated)
	}

	otherCtx := newResourceCtx("PATCH", "/api/portfolios/"+other.ID.String(), map[string]any{"name": "stolen"})
	otherCtx.Params = param.Params{{Key: "id", Value: other.ID.String()}}
	UpdatePortfolio(db)(context.Background(), otherCtx)
	if otherCtx.Response.StatusCode() != 404 {
		t.Fatalf("cross-user update: expected 404, got %d", otherCtx.Response.StatusCode())
	}
}

func TestPortfolios_ValidationAndDefaultProtection(t *testing.T) {
	db := setupTestDB(t)

	createCtx := newResourceCtx("POST", "/api/portfolios", map[string]any{"name": ""})
	CreatePortfolio(db)(context.Background(), createCtx)
	if createCtx.Response.StatusCode() != 400 {
		t.Fatalf("empty name: expected 400, got %d", createCtx.Response.StatusCode())
	}

	emptyUpdateCtx := newResourceCtx("PATCH", "/api/portfolios/"+testPortfolioID.String(), map[string]any{})
	emptyUpdateCtx.Params = param.Params{{Key: "id", Value: testPortfolioID.String()}}
	UpdatePortfolio(db)(context.Background(), emptyUpdateCtx)
	if emptyUpdateCtx.Response.StatusCode() != 400 {
		t.Fatalf("empty update: expected 400, got %d", emptyUpdateCtx.Response.StatusCode())
	}

	deleteCtx := newResourceCtx("DELETE", "/api/portfolios/"+testPortfolioID.String(), nil)
	deleteCtx.Params = param.Params{{Key: "id", Value: testPortfolioID.String()}}
	DeletePortfolio(db)(context.Background(), deleteCtx)
	if deleteCtx.Response.StatusCode() != 400 {
		t.Fatalf("delete default: expected 400, got %d", deleteCtx.Response.StatusCode())
	}
}

func TestDeletePortfolio_CleansRelatedData(t *testing.T) {
	db := setupTestDB(t)
	portfolio := models.Portfolio{ID: uuid.New(), UserID: testUserID, Name: "Disposable"}
	if err := db.Create(&portfolio).Error; err != nil {
		t.Fatal(err)
	}
	holding := models.Holding{
		ID: uuid.New(), UserID: testUserID, PortfolioID: portfolio.ID,
		AssetId: "stocks", Symbol: "DELETE",
	}
	lot := models.HoldingLot{ID: uuid.New(), HoldingID: holding.ID, Shares: decimal.NewFromInt(1)}
	record := models.PortfolioRecord{
		ID: uuid.New(), UserID: testUserID, PortfolioID: portfolio.ID,
		Timestamp: time.Now(), Assets: models.AssetMapColumn{}, Holdings: models.HoldingSnapshotColumn{},
	}
	fund := models.AvailableFund{
		ID: uuid.New(), UserID: testUserID, PortfolioID: portfolio.ID,
		Currency: "USD", Amount: decimal.NewFromInt(100),
	}
	transaction := models.FundTransaction{
		ID: uuid.New(), UserID: testUserID, PortfolioID: portfolio.ID,
		Type: "deposit", Amount: decimal.NewFromInt(100), Currency: "USD",
	}
	if err := db.Create(&holding).Error; err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{&lot, &record, &fund, &transaction} {
		if err := db.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}

	deleteCtx := newResourceCtx("DELETE", "/api/portfolios/"+portfolio.ID.String(), nil)
	deleteCtx.Params = param.Params{{Key: "id", Value: portfolio.ID.String()}}
	DeletePortfolio(db)(context.Background(), deleteCtx)
	if deleteCtx.Response.StatusCode() != 200 {
		t.Fatalf("delete: expected 200, got %d: %s", deleteCtx.Response.StatusCode(), deleteCtx.Response.Body())
	}

	checks := []struct {
		model any
		where string
		arg   any
	}{
		{&models.Portfolio{}, "id = ?", portfolio.ID},
		{&models.Holding{}, "portfolio_id = ?", portfolio.ID},
		{&models.HoldingLot{}, "holding_id = ?", holding.ID},
		{&models.PortfolioRecord{}, "portfolio_id = ?", portfolio.ID},
		{&models.AvailableFund{}, "portfolio_id = ?", portfolio.ID},
		{&models.FundTransaction{}, "portfolio_id = ?", portfolio.ID},
	}
	for _, check := range checks {
		var count int64
		if err := db.Model(check.model).Where(check.where, check.arg).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("expected related %T rows deleted, count=%d", check.model, count)
		}
	}
}
