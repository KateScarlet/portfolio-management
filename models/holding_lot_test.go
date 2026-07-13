package models

import (
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupModelTestDB(t *testing.T) *gorm.DB {
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
	schema := "test_models_" + uuid.NewString()
	if err := adminDB.Exec(fmt.Sprintf(`CREATE SCHEMA %q`, schema)).Error; err != nil {
		adminSQLDB.Close() //nolint:errcheck // best effort after setup failure
		t.Fatal(err)
	}

	testURL, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := testURL.Query()
	query.Set("search_path", schema)
	testURL.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(testURL.String()), &gorm.Config{})
	if err != nil {
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
	if err := db.AutoMigrate(&HoldingLot{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestHoldingLotRepository_Lifecycle(t *testing.T) {
	db := setupModelTestDB(t)
	firstHoldingID := uuid.New()
	secondHoldingID := uuid.New()
	early := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
	late := early.Add(time.Minute)
	lateLot := HoldingLot{
		ID: uuid.New(), HoldingID: firstHoldingID, Date: late,
		Shares: decimal.NewFromInt(2), Cost: decimal.NewFromInt(200),
	}
	if err := CreateLot(db, &lateLot); err != nil {
		t.Fatal(err)
	}
	earlyLot := HoldingLot{
		ID: uuid.New(), HoldingID: firstHoldingID, Date: early,
		Shares: decimal.NewFromInt(1), Cost: decimal.NewFromInt(100),
	}
	secondHoldingLots := []HoldingLot{
		{ID: uuid.New(), HoldingID: secondHoldingID, Date: early, Shares: decimal.NewFromInt(3)},
		{ID: uuid.New(), HoldingID: secondHoldingID, Date: late, Shares: decimal.NewFromInt(4)},
	}
	if err := CreateLots(db, append([]HoldingLot{earlyLot}, secondHoldingLots...)); err != nil {
		t.Fatal(err)
	}
	if err := CreateLots(db, nil); err != nil {
		t.Fatalf("empty batch should be a no-op: %v", err)
	}

	firstLots, err := LoadLots(db, firstHoldingID)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstLots) != 2 || firstLots[0].ID != earlyLot.ID || firstLots[1].ID != lateLot.ID {
		t.Fatalf("expected lots ordered by date, got %+v", firstLots)
	}
	grouped, err := LoadLotsByHoldingIDs(db, []uuid.UUID{firstHoldingID, secondHoldingID})
	if err != nil {
		t.Fatal(err)
	}
	if len(grouped[firstHoldingID]) != 2 || len(grouped[secondHoldingID]) != 2 {
		t.Fatalf("unexpected grouped lots: %+v", grouped)
	}
	empty, err := LoadLotsByHoldingIDs(db, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty holding IDs should return an empty map, got %+v, err=%v", empty, err)
	}

	replacement := []HoldingLot{
		{ID: uuid.New(), HoldingID: uuid.New(), Date: late.Add(time.Minute), Shares: decimal.NewFromInt(5)},
		{ID: uuid.New(), HoldingID: uuid.New(), Date: late.Add(2 * time.Minute), Shares: decimal.NewFromInt(6)},
	}
	if err := ReplaceLots(db, firstHoldingID, replacement); err != nil {
		t.Fatal(err)
	}
	firstLots, err = LoadLots(db, firstHoldingID)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstLots) != 2 || firstLots[0].HoldingID != firstHoldingID || firstLots[1].HoldingID != firstHoldingID {
		t.Fatalf("replacement must assign the requested holding ID, got %+v", firstLots)
	}

	if err := DeleteLotByID(db, firstLots[0].ID); err != nil {
		t.Fatal(err)
	}
	firstLots, _ = LoadLots(db, firstHoldingID)
	if len(firstLots) != 1 {
		t.Fatalf("expected one lot after deleting by ID, got %d", len(firstLots))
	}
	if err := DeleteLotsByHoldingID(db, secondHoldingID); err != nil {
		t.Fatal(err)
	}
	secondLots, _ := LoadLots(db, secondHoldingID)
	if len(secondLots) != 0 {
		t.Fatalf("expected all second holding lots deleted, got %d", len(secondLots))
	}
	if err := ReplaceLots(db, firstHoldingID, nil); err != nil {
		t.Fatal(err)
	}
	firstLots, _ = LoadLots(db, firstHoldingID)
	if len(firstLots) != 0 {
		t.Fatalf("empty replacement should clear existing lots, got %d", len(firstLots))
	}
}

func TestReplaceLots_RollsBackWhenInsertFails(t *testing.T) {
	db := setupModelTestDB(t)
	holdingID := uuid.New()
	original := HoldingLot{ID: uuid.New(), HoldingID: holdingID, Shares: decimal.NewFromInt(1)}
	if err := CreateLot(db, &original); err != nil {
		t.Fatal(err)
	}
	duplicateID := uuid.New()
	err := ReplaceLots(db, holdingID, []HoldingLot{
		{ID: duplicateID, Shares: decimal.NewFromInt(2)},
		{ID: duplicateID, Shares: decimal.NewFromInt(3)},
	})
	if err == nil {
		t.Fatal("expected duplicate primary key insertion to fail")
	}
	lots, loadErr := LoadLots(db, holdingID)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(lots) != 1 || lots[0].ID != original.ID {
		t.Fatalf("failed replacement must preserve original lots, got %+v", lots)
	}
}
