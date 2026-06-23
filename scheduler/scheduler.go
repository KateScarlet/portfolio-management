package scheduler

import (
	"fmt"
	"log/slog"
	"portfolio-management/models"
	"portfolio-management/yahoo"
	"strconv"
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
	db       *gorm.DB
	ticker   *time.Ticker
	stopCh   chan struct{}
	mu       sync.RWMutex
	users    map[string]*userSyncState
	notifier *Notifier
}

type userSyncState struct {
	interval    time.Duration
	lastSyncAt  time.Time
	lastSyncErr string
	syncing     bool
	mu          sync.Mutex
}

func New(db *gorm.DB) *PriceScheduler {
	s := &PriceScheduler{
		db:     db,
		stopCh: make(chan struct{}),
		users:  make(map[string]*userSyncState),
	}
	s.Start()
	return s
}

func (s *PriceScheduler) Start() {
	s.mu.Lock()
	if s.ticker != nil {
		s.mu.Unlock()
		return
	}
	slog.Info("scheduler starting")
	s.ticker = time.NewTicker(1 * time.Minute)
	stopCh := s.stopCh
	ticker := s.ticker
	s.mu.Unlock()

	go s.run(ticker, stopCh)
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
	s.mu.Unlock()
	slog.Info("scheduler stopped")
}

func (s *PriceScheduler) run(ticker *time.Ticker, stopCh <-chan struct{}) {
	for {
		select {
		case <-ticker.C:
			s.syncAllUsers()
		case <-stopCh:
			return
		}
	}
}

func (s *PriceScheduler) syncAllUsers() {
	var userIDs []string
	if err := s.db.Model(&models.Holding{}).Distinct("user_id").Pluck("user_id", &userIDs).Error; err != nil {
		slog.Error("failed to query user IDs", "error", err)
		return
	}

	for _, userID := range userIDs {
		var setting models.Setting
		if err := s.db.Where("`key` = ? AND user_id = ?", "syncInterval", userID).First(&setting).Error; err != nil {
			continue
		}

		interval, err := strconv.Atoi(setting.Value)
		if err != nil || interval <= 0 {
			continue
		}

		s.mu.Lock()
		state, exists := s.users[userID]
		if !exists {
			state = &userSyncState{
				interval: time.Duration(interval) * time.Minute,
			}
			s.users[userID] = state
		} else {
			state.mu.Lock()
			state.interval = time.Duration(interval) * time.Minute
			state.mu.Unlock()
		}
		s.mu.Unlock()

		state.mu.Lock()
		shouldSync := time.Since(state.lastSyncAt) >= state.interval && !state.syncing
		state.mu.Unlock()

		if shouldSync {
			go s.syncUser(userID, state)
		}
	}
}

// syncResult holds the outcome of a single price fetch.
type syncResult struct {
	holding *models.Holding
	result  *yahoo.PriceResult
	err     error
}

const (
	maxConcurrentFetch = 5
	fetchRateLimit     = 50 * time.Millisecond // min interval between API calls
)

func (s *PriceScheduler) syncUser(userID string, state *userSyncState) {
	state.mu.Lock()
	if state.syncing {
		state.mu.Unlock()
		return
	}
	state.syncing = true
	state.mu.Unlock()

	defer func() {
		state.mu.Lock()
		state.syncing = false
		state.mu.Unlock()
	}()

	slog.Info("starting price sync", "userId", userID)

	var holdings []models.Holding
	if err := s.db.Where("user_id = ? AND symbol != ''", userID).Find(&holdings).Error; err != nil {
		state.mu.Lock()
		state.lastSyncErr = err.Error()
		state.mu.Unlock()
		slog.Error("failed to query holdings", "userId", userID, "error", err)
		return
	}

	if len(holdings) == 0 {
		slog.Info("no holdings with symbols to sync", "userId", userID)
		state.mu.Lock()
		state.lastSyncAt = time.Now()
		state.lastSyncErr = ""
		state.mu.Unlock()
		return
	}

	synced := 0
	failed := 0
	syncedPrices := make(map[string]float64)

	// Concurrent price fetching with rate limiting.
	sem := make(chan struct{}, maxConcurrentFetch)
	results := make(chan syncResult, len(holdings))

	var wg sync.WaitGroup
	for i := range holdings {
		wg.Add(1)
		sem <- struct{}{} // acquire concurrency slot (blocks if max reached)
		go func(h *models.Holding) {
			defer func() {
				<-sem // release concurrency slot
				wg.Done()
			}()
			result, err := yahoo.FetchQuote(h.Symbol)
			results <- syncResult{holding: h, result: result, err: err}
		}(&holdings[i])
		time.Sleep(fetchRateLimit) // rate limit between dispatches
	}

	// Close results channel once all goroutines finish.
	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.err != nil {
			slog.Error("failed to fetch price", "userId", userID, "name", r.holding.Name, "symbol", r.holding.Symbol, "error", r.err)
			failed++
			continue
		}

		updates := map[string]any{
			"price": r.result.Price,
			"value": r.holding.Shares * r.result.Price,
		}
		if err := s.db.Model(&models.Holding{}).Where("id = ? AND user_id = ?", r.holding.ID, userID).Updates(updates).Error; err != nil {
			slog.Error("failed to update holding", "userId", userID, "id", r.holding.ID, "error", err)
			failed++
			continue
		}

		synced++
		syncedPrices[r.holding.Symbol] = r.result.Price
		slog.Info("synced holding", "userId", userID, "name", r.holding.Name, "symbol", r.holding.Symbol, "price", r.result.Price)
	}

	state.mu.Lock()
	state.lastSyncAt = time.Now()
	if failed > 0 {
		state.lastSyncErr = fmt.Sprintf("%d/%d failed", failed, synced+failed)
	} else {
		state.lastSyncErr = ""
	}
	state.mu.Unlock()

	slog.Info("sync completed", "userId", userID, "synced", synced, "failed", failed)

	if s.notifier != nil {
		s.notifier.NotifyAfterSync(userID, holdings, syncedPrices)
	}
}

func (s *PriceScheduler) GetStatusForUser(userID string) SyncStatus {
	s.mu.RLock()
	state, exists := s.users[userID]
	s.mu.RUnlock()

	if !exists {
		return SyncStatus{}
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	return SyncStatus{
		LastSyncAt:  state.lastSyncAt,
		LastSyncErr: state.lastSyncErr,
		Syncing:     state.syncing,
	}
}

func (s *PriceScheduler) TriggerSyncForUser(userID string) bool {
	s.mu.Lock()
	state, exists := s.users[userID]
	if !exists {
		state = &userSyncState{}
		s.users[userID] = state
	}
	s.mu.Unlock()

	go s.syncUser(userID, state)
	return true
}

// TriggerSyncForUserSync runs sync synchronously and returns the final status.
func (s *PriceScheduler) TriggerSyncForUserSync(userID string) (SyncStatus, bool) {
	s.mu.Lock()
	state, exists := s.users[userID]
	if !exists {
		state = &userSyncState{}
		s.users[userID] = state
	}
	s.mu.Unlock()

	s.syncUser(userID, state)

	state.mu.Lock()
	defer state.mu.Unlock()
	return SyncStatus{
		LastSyncAt:  state.lastSyncAt,
		LastSyncErr: state.lastSyncErr,
		Syncing:     false,
	}, true
}

func (s *PriceScheduler) SetNotifier(n *Notifier) {
	s.notifier = n
}
