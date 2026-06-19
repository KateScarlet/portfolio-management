package scheduler

import (
	"fmt"
	"log"
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
	defer s.mu.Unlock()
	if s.ticker != nil {
		return
	}
	log.Printf("[scheduler] starting price sync every %v", s.interval)
	s.ticker = time.NewTicker(s.interval)
	go s.run()
}

func (s *PriceScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ticker != nil {
		s.ticker.Stop()
		s.ticker = nil
		close(s.stopCh)
		s.stopCh = make(chan struct{})
		log.Printf("[scheduler] stopped")
	}
}

func (s *PriceScheduler) UpdateInterval(interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interval = interval
	if interval <= 0 {
		if s.ticker != nil {
			s.ticker.Stop()
			s.ticker = nil
			close(s.stopCh)
			s.stopCh = make(chan struct{})
			log.Printf("[scheduler] disabled (interval=0)")
		}
		return
	}
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.ticker = time.NewTicker(interval)
	go s.run()
	log.Printf("[scheduler] interval updated to %v", interval)
}

func (s *PriceScheduler) run() {
	for {
		select {
		case <-s.ticker.C:
			s.SyncNow()
		case <-s.stopCh:
			return
		}
	}
}

func (s *PriceScheduler) SyncNow() {
	s.mu.Lock()
	if s.syncing {
		s.mu.Unlock()
		log.Printf("[scheduler] sync already in progress, skipping")
		return
	}
	s.syncing = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.syncing = false
		s.mu.Unlock()
	}()

	log.Printf("[scheduler] starting price sync...")

	var holdings []models.Holding
	if err := s.db.Where("symbol != ''").Find(&holdings).Error; err != nil {
		s.mu.Lock()
		s.lastSyncErr = err.Error()
		s.mu.Unlock()
		log.Printf("[scheduler] failed to query holdings: %v", err)
		return
	}

	if len(holdings) == 0 {
		log.Printf("[scheduler] no holdings with symbols to sync")
		s.mu.Lock()
		s.lastSyncAt = time.Now()
		s.lastSyncErr = ""
		s.mu.Unlock()
		return
	}

	synced := 0
	failed := 0

	for i, h := range holdings {
		if i > 0 {
			time.Sleep(200 * time.Millisecond)
		}

		result, err := yahoo.FetchQuote(h.Symbol)
		if err != nil {
			log.Printf("[scheduler] failed to fetch price for %s (%s): %v", h.Name, h.Symbol, err)
			failed++
			continue
		}

		updates := map[string]interface{}{
			"price": result.Price,
			"value": h.Shares * result.Price,
		}
		if err := s.db.Model(&models.Holding{}).Where("id = ?", h.ID).Updates(updates).Error; err != nil {
			log.Printf("[scheduler] failed to update holding %s: %v", h.ID, err)
			failed++
			continue
		}

		synced++
		log.Printf("[scheduler] synced %s (%s): price=%.2f", h.Name, h.Symbol, result.Price)
	}

	s.mu.Lock()
	s.lastSyncAt = time.Now()
	if failed > 0 {
		s.lastSyncErr = fmt.Sprintf("%d/%d failed", failed, synced+failed)
	} else {
		s.lastSyncErr = ""
	}
	s.mu.Unlock()

	log.Printf("[scheduler] sync completed: %d synced, %d failed", synced, failed)
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
