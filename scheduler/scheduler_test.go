package scheduler

import (
	"portfolio-management/marketsource"
	"portfolio-management/models"
	"sync"
	"testing"
	"time"

	"github.com/libtnb/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
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
	if s.ticker != nil {
		t.Error("expected ticker to be nil after Stop")
	}
}

func TestPriceScheduler_GetStatusForPortfolio_Unknown(t *testing.T) {
	db := setupTestDB(t)
	s := New(db, setupTestRouter(db))
	s.Stop()
	status := s.GetStatusForPortfolio("nonexistent", "nonexistent-portfolio")
	if status.Syncing {
		t.Error("expected Syncing to be false")
	}
	if !status.LastSyncAt.IsZero() {
		t.Error("expected LastSyncAt to be zero")
	}
}

func TestPriceScheduler_StopIdempotent(t *testing.T) {
	db := setupTestDB(t)
	s := New(db, setupTestRouter(db))
	s.Stop()
	s.Stop()
	if s.ticker != nil {
		t.Error("expected ticker to be nil after double Stop")
	}
}

func TestPriceScheduler_TriggerSyncForPortfolio(t *testing.T) {
	db := setupTestDB(t)
	s := New(db, setupTestRouter(db))
	s.Stop()

	if !s.TriggerSyncForPortfolio("user1", "portfolio1") {
		t.Error("expected first trigger to succeed")
	}
	time.Sleep(50 * time.Millisecond)

	status := s.GetStatusForPortfolio("user1", "portfolio1")
	if status.LastSyncAt.IsZero() {
		t.Error("expected LastSyncAt to be set after sync")
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
			s.TriggerSyncForPortfolio("concurrent-user", "concurrent-portfolio")
		}()
	}

	wg.Wait()

	s.mu.RLock()
	count := len(s.states)
	_, exists := s.states[syncKey("concurrent-user", "concurrent-portfolio")]
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
			s.TriggerSyncForPortfolioSync("sync-user", "sync-portfolio")
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
			s.TriggerSyncForPortfolio("user-"+string(rune('A'+idx%26)), "portfolio-"+string(rune('A'+idx%26)))
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
