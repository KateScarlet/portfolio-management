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
	doneCh      chan struct{} // closed when the run goroutine exits
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
		doneCh:   make(chan struct{}),
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
	ticker := s.ticker
	s.doneCh = make(chan struct{})
	doneCh := s.doneCh
	s.mu.Unlock()

	go s.run(ticker, stopCh, doneCh)
}

func (s *PriceScheduler) Stop() {
	s.mu.Lock()
	if s.ticker == nil {
		s.mu.Unlock()
		return
	}
	s.ticker.Stop()
	s.ticker = nil
	if s.stopCh != nil {
		close(s.stopCh)
	}
	s.stopCh = make(chan struct{})
	oldDoneCh := s.doneCh
	s.doneCh = make(chan struct{})
	s.mu.Unlock()

	if oldDoneCh != nil {
		<-oldDoneCh
	}
	slog.Info("scheduler stopped")
}

func (s *PriceScheduler) UpdateInterval(interval time.Duration) {
	s.mu.Lock()
	s.interval = interval
	if interval <= 0 {
		s.mu.Unlock()
		s.Stop()
		slog.Info("scheduler disabled", "interval", 0)
		return
	}

	var oldDoneCh chan struct{}
	if s.ticker != nil {
		s.ticker.Stop()
		if s.stopCh != nil {
			close(s.stopCh)
		}
		oldDoneCh = s.doneCh
	}
	s.ticker = time.NewTicker(interval)
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	ticker := s.ticker
	stopCh := s.stopCh
	doneCh := s.doneCh
	s.mu.Unlock()

	if oldDoneCh != nil {
		<-oldDoneCh
	}

	go s.run(ticker, stopCh, doneCh)
	slog.Info("scheduler interval updated", "interval", interval)
}

func (s *PriceScheduler) run(ticker *time.Ticker, stopCh <-chan struct{}, doneCh chan struct{}) {
	defer close(doneCh)
	for {
		select {
		case <-ticker.C:
			s.SyncNow()
		case <-stopCh:
			return
		}
	}
}

// TryStartSync atomically attempts to start a sync.
// Returns true if sync was started, false if already syncing.
func (s *PriceScheduler) TryStartSync() bool {
	s.mu.Lock()
	if s.syncing {
		s.mu.Unlock()
		return false
	}
	s.syncing = true
	s.mu.Unlock()

	go s.doSync()
	return true
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

	s.doSync()
}

// doSync performs the actual sync and resets the syncing flag when done.
func (s *PriceScheduler) doSync() {
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
