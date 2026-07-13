package scheduler

import (
	"fmt"
	"os"
	"portfolio-management/internal/marketsource"
	"portfolio-management/models"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		dsn = "postgres://localhost:5432/portfolio_test?sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Portfolio{}, &models.Holding{}, &models.Setting{}, &models.User{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func setupTestRouter(db *gorm.DB) *marketsource.Router {
	return marketsource.NewRouter(db, map[string]marketsource.MarketSource{})
}

func TestSyncStatus(t *testing.T) {
	status := SyncStatus{
		LastSyncAt: time.Now(),
	}
	if status.Syncing {
		t.Error("expected Syncing to be false")
	}
	if status.LastSyncAt.IsZero() {
		t.Error("expected LastSyncAt to be set")
	}
}

func TestPriceScheduler_New(t *testing.T) {
	db := setupTestDB(t)
	s := New(db, setupTestRouter(db))
	s.Stop()
	if !s.stopped {
		t.Error("expected scheduler to be stopped after Stop")
	}
}

func TestPriceScheduler_StopIdempotent(t *testing.T) {
	db := setupTestDB(t)
	s := New(db, setupTestRouter(db))
	s.Stop()
	s.Stop()
	if !s.stopped {
		t.Error("expected scheduler to be stopped after double Stop")
	}
}

func TestPriceScheduler_TriggerSyncForPortfolio(t *testing.T) {
	db := setupTestDB(t)
	s := New(db, setupTestRouter(db))
	s.Stop()

	if !s.TriggerSyncForPortfolio(uuid.MustParse("00000000-0000-0000-0000-000000000001"), uuid.MustParse("00000000-0000-0000-0000-000000000010")) {
		t.Error("expected first trigger to succeed")
	}
}

func TestPriceScheduler_ConcurrentTriggerSync_NoDuplicateStates(t *testing.T) {
	db := setupTestDB(t)
	s := New(db, setupTestRouter(db))
	s.Stop()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			s.TriggerSyncForPortfolio(uuid.MustParse("00000000-0000-0000-0000-000000000099"), uuid.MustParse("00000000-0000-0000-0000-000000000099"))
		}()
	}

	wg.Wait()

	s.mu.RLock()
	count := len(s.states)
	_, exists := s.states[syncKey(uuid.MustParse("00000000-0000-0000-0000-000000000099"), uuid.MustParse("00000000-0000-0000-0000-000000000099"))]
	s.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected exactly 1 state entry, got %d (race condition!)", count)
	}
	if !exists {
		t.Error("expected state entry to exist")
	}
}

func TestPriceScheduler_ConcurrentTriggerSyncForPortfolioSync_NoDuplicateStates(t *testing.T) {
	db := setupTestDB(t)
	s := New(db, setupTestRouter(db))
	s.Stop()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()
			s.TriggerSyncForPortfolioSync(uuid.MustParse("00000000-0000-0000-0000-000000000088"), uuid.MustParse("00000000-0000-0000-0000-000000000088"))
		}()
	}

	wg.Wait()

	s.mu.RLock()
	count := len(s.states)
	s.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected exactly 1 state entry, got %d (race condition!)", count)
	}
}

func TestPriceScheduler_ConcurrentDifferentPortfolios(t *testing.T) {
	db := setupTestDB(t)
	s := New(db, setupTestRouter(db))
	s.Stop()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			id := uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012d", idx%26+1))
			s.TriggerSyncForPortfolio(id, id)
		}(i)
	}

	wg.Wait()

	s.mu.RLock()
	count := len(s.states)
	s.mu.RUnlock()

	if count > 26 {
		t.Errorf("expected at most 26 unique entries, got %d", count)
	}
}

func TestPriceScheduler_SetNotifier(t *testing.T) {
	db := setupTestDB(t)
	s := New(db, setupTestRouter(db))
	s.Stop()

	n := NewNotifier(db, setupTestRouter(db))
	s.SetNotifier(n)
	if s.notifier != n {
		t.Error("expected notifier to be set")
	}
}
