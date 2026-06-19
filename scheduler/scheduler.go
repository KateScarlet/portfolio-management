package scheduler

import (
	"fmt"
	"log/slog"
	"permanent-portfolio/models"
	"permanent-portfolio/yahoo"
	"sync"
	"time"

	"gorm.io/gorm"
)

type SyncStatus struct {
	LastSyncAt  time.Time `json:"lastSyncAt"`
	LastSyncErr string    `json:"lastSyncErr,omitempty"`
	Syncing     bool      `json:"syncing"`
}

type PriceScheduler struct {
	db          *gorm.DB
	interval    time.Duration
	ticker      *time.Ticker
	stopCh      chan struct{}
	mu          sync.RWMutex
	lastSyncAt  time.Time
	lastSyncErr string
	syncing     bool
}

func New(db *gorm.DB, interval time.Duration) *PriceScheduler {
	s := &PriceScheduler{
		db:       db,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
	if interval > 0 {
		s.Start()
	}
	return s
}

func (s *PriceScheduler) Start() {
	s.mu.Lock()
	if s.ticker != nil {
		s.mu.Unlock()
		return
	}
	slog.Info("scheduler starting price sync", "interval", s.interval)
	s.ticker = time.NewTicker(s.interval)
	stopCh := s.stopCh
	s.mu.Unlock()

	go s.run(stopCh)
}

func (s *PriceScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ticker != nil {
		s.ticker.Stop()
		s.ticker = nil
		close(s.stopCh)
		s.stopCh = make(chan struct{})
		slog.Info("scheduler stopped")
	}
}

func (s *PriceScheduler) UpdateInterval(interval time.Duration) {
	s.mu.Lock()
	s.interval = interval
	if interval <= 0 {
		if s.ticker != nil {
			s.ticker.Stop()
			s.ticker = nil
			close(s.stopCh)
			s.stopCh = make(chan struct{})
			slog.Info("scheduler disabled", "interval", 0)
		}
		s.mu.Unlock()
		return
	}
	if s.ticker != nil {
		s.ticker.Stop()
		close(s.stopCh)
	}
	s.ticker = time.NewTicker(interval)
	s.stopCh = make(chan struct{})
	stopCh := s.stopCh
	s.mu.Unlock()

	go s.run(stopCh)
	slog.Info("scheduler interval updated", "interval", interval)
}

func (s *PriceScheduler) run(stopCh <-chan struct{}) {
	for {
		select {
		case <-s.ticker.C:
			s.SyncNow()
		case <-stopCh:
			return
		}
	}
}

func (s *PriceScheduler) SyncNow() {
	s.mu.Lock()
	if s.syncing {
		s.mu.Unlock()
		slog.Info("sync already in progress, skipping")
		return
	}
	s.syncing = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.syncing = false
		s.mu.Unlock()
	}()

	slog.Info("starting price sync")

	var holdings []models.Holding
	if err := s.db.Where("symbol != ''").Find(&holdings).Error; err != nil {
		s.mu.Lock()
		s.lastSyncErr = err.Error()
		s.mu.Unlock()
		slog.Error("failed to query holdings", "error", err)
		return
	}

	if len(holdings) == 0 {
		slog.Info("no holdings with symbols to sync")
		s.mu.Lock()
		s.lastSyncAt = time.Now()
		s.lastSyncErr = ""
		s.mu.Unlock()
		return
	}

	synced := 0
	failed := 0

	for i := range holdings {
		if i > 0 {
			time.Sleep(200 * time.Millisecond)
		}

		h := &holdings[i]
		result, err := yahoo.FetchQuote(h.Symbol)
		if err != nil {
			slog.Error("failed to fetch price", "name", h.Name, "symbol", h.Symbol, "error", err)
			failed++
			continue
		}

		updates := map[string]interface{}{
			"price": result.Price,
			"value": h.Shares * result.Price,
		}
		if err := s.db.Model(&models.Holding{}).Where("id = ?", h.ID).Updates(updates).Error; err != nil {
			slog.Error("failed to update holding", "id", h.ID, "error", err)
			failed++
			continue
		}

		synced++
		slog.Info("synced holding", "name", h.Name, "symbol", h.Symbol, "price", result.Price)
	}

	s.mu.Lock()
	s.lastSyncAt = time.Now()
	if failed > 0 {
		s.lastSyncErr = fmt.Sprintf("%d/%d failed", failed, synced+failed)
	} else {
		s.lastSyncErr = ""
	}
	s.mu.Unlock()

	slog.Info("sync completed", "synced", synced, "failed", failed)
}

func (s *PriceScheduler) GetStatus() SyncStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := SyncStatus{
		LastSyncAt: s.lastSyncAt,
		Syncing:    s.syncing,
	}
	if s.lastSyncErr != "" {
		status.LastSyncErr = s.lastSyncErr
	}
	return status
}
