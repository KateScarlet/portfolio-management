package scheduler

import (
	"testing"
	"time"
)

func TestSyncStatus(t *testing.T) {
	status := SyncStatus{
		LastSyncAt:  time.Now(),
		LastSyncErr: "",
		Syncing:     false,
	}
	if status.Syncing {
		t.Error("expected Syncing to be false")
	}
	if status.LastSyncAt.IsZero() {
		t.Error("expected LastSyncAt to be set")
	}
}

func TestPriceScheduler_New(t *testing.T) {
	s := New(nil)
	s.Stop()
	if s.ticker != nil {
		t.Error("expected ticker to be nil after Stop")
	}
}

func TestPriceScheduler_GetStatusForUser_Unknown(t *testing.T) {
	s := New(nil)
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
	s := New(nil)
	s.Stop()
	s.Stop()
	if s.ticker != nil {
		t.Error("expected ticker to be nil after double Stop")
	}
}

func TestPriceScheduler_TriggerSyncForUser(t *testing.T) {
	s := New(nil)
	s.Stop()

	if !s.TriggerSyncForUser("user1") {
		t.Error("expected first trigger to succeed")
	}

	status := s.GetStatusForUser("user1")
	if !status.LastSyncAt.IsZero() {
		t.Error("expected LastSyncAt to be zero since db is nil")
	}
}
