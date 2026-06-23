package scheduler

import (
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
	if err := db.AutoMigrate(&models.Holding{}, &models.Setting{}, &models.User{}); err != nil {
		t.Fatal(err)
	}
	return db
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
	s := New(db)
	s.Stop()
	if s.ticker != nil {
		t.Error("expected ticker to be nil after Stop")
	}
}

func TestPriceScheduler_GetStatusForUser_Unknown(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	s.Stop()
	status := s.GetStatusForUser("nonexistent")
	if status.Syncing {
		t.Error("expected Syncing to be false")
	}
	if !status.LastSyncAt.IsZero() {
		t.Error("expected LastSyncAt to be zero")
	}
}

func TestPriceScheduler_StopIdempotent(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	s.Stop()
	s.Stop()
	if s.ticker != nil {
		t.Error("expected ticker to be nil after double Stop")
	}
}

func TestPriceScheduler_TriggerSyncForUser(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	s.Stop()

	if !s.TriggerSyncForUser("user1") {
		t.Error("expected first trigger to succeed")
	}
	// Wait for the async goroutine to finish
	time.Sleep(50 * time.Millisecond)

	status := s.GetStatusForUser("user1")
	if status.LastSyncAt.IsZero() {
		t.Error("expected LastSyncAt to be set after sync")
	}
}

func TestPriceScheduler_ConcurrentTriggerSync_NoDuplicateStates(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	s.Stop()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			s.TriggerSyncForUser("concurrent-user")
		}()
	}

	wg.Wait()

	s.mu.RLock()
	count := len(s.users)
	_, exists := s.users["concurrent-user"]
	s.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected exactly 1 user entry, got %d (race condition!)", count)
	}
	if !exists {
		t.Error("expected user entry to exist")
	}
}

func TestPriceScheduler_ConcurrentTriggerSyncForUserSync_NoDuplicateStates(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	s.Stop()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			s.TriggerSyncForUserSync("sync-user")
		}()
	}

	wg.Wait()

	s.mu.RLock()
	count := len(s.users)
	s.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected exactly 1 user entry, got %d (race condition!)", count)
	}
}

func TestPriceScheduler_ConcurrentDifferentUsers(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	s.Stop()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			s.TriggerSyncForUser("user-" + string(rune('A'+idx%26)))
		}(i)
	}

	wg.Wait()

	s.mu.RLock()
	count := len(s.users)
	s.mu.RUnlock()

	if count > 26 {
		t.Errorf("expected at most 26 unique users, got %d", count)
	}
}

func TestPriceScheduler_SetNotifier(t *testing.T) {
	db := setupTestDB(t)
	s := New(db)
	s.Stop()

	n := NewNotifier(db)
	s.SetNotifier(n)
	if s.notifier != n {
		t.Error("expected notifier to be set")
	}
}
