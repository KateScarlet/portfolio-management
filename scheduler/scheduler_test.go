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

func TestPriceScheduler_NewDisabled(t *testing.T) {
	s := New(nil, 0)
	if s.interval != 0 {
		t.Errorf("expected interval 0, got %v", s.interval)
	}
	if s.ticker != nil {
		t.Error("expected ticker to be nil when interval is 0")
	}
}

func TestPriceScheduler_UpdateIntervalToZero(t *testing.T) {
	s := New(nil, 60*time.Minute)
	if s.ticker == nil {
		t.Fatal("expected ticker to be set")
	}
	s.Stop()
	s.UpdateInterval(0)
	if s.ticker != nil {
		t.Error("expected ticker to be nil after disabling")
	}
}

func TestPriceScheduler_UpdateIntervalToZeroWithoutStop(t *testing.T) {
	s := New(nil, 60*time.Minute)
	if s.ticker == nil {
		t.Fatal("expected ticker to be set")
	}
	s.UpdateInterval(0)
	if s.ticker != nil {
		t.Error("expected ticker to be nil after UpdateInterval(0)")
	}
}

func TestPriceScheduler_GetStatus(t *testing.T) {
	s := New(nil, 0)
	status := s.GetStatus()
	if status.Syncing {
		t.Error("expected Syncing to be false")
	}
	if !status.LastSyncAt.IsZero() {
		t.Error("expected LastSyncAt to be zero")
	}
}

func TestPriceScheduler_StopIdempotent(t *testing.T) {
	s := New(nil, 60*time.Minute)
	s.Stop()
	s.Stop()
	if s.ticker != nil {
		t.Error("expected ticker to be nil after double Stop")
	}
}

func TestPriceScheduler_UpdateIntervalRestart(t *testing.T) {
	s := New(nil, 60*time.Minute)
	if s.ticker == nil {
		t.Fatal("expected ticker to be set")
	}
	s.UpdateInterval(30 * time.Minute)
	if s.interval != 30*time.Minute {
		t.Errorf("expected interval 30m, got %v", s.interval)
	}
	if s.ticker == nil {
		t.Error("expected ticker to be set after restart")
	}
	s.Stop()
}
