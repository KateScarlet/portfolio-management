package scheduler

import (
	"fmt"
	"log/slog"
	"portfolio-management/marketsource"
	"portfolio-management/models"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type SyncStatus struct {
	LastSyncAt  time.Time `json:"lastSyncAt"`
	LastSyncErr string    `json:"lastSyncErr,omitempty"`
	Syncing     bool      `json:"syncing"`
}

type PriceScheduler struct {
	db       *gorm.DB
	router   *marketsource.Router
	mu       sync.RWMutex
	states   map[string]*syncState
	notifier *Notifier
	eventBus *EventBus
	stopCh   chan struct{}
	stopped  bool
}

type syncState struct {
	interval        time.Duration
	lastSyncAt      time.Time
	lastSyncErr     string
	syncing         bool
	timer           *time.Timer
	scheduleVersion uint64
	mu              sync.Mutex
}

func syncKey(userID, portfolioID uuid.UUID) string {
	return userID.String() + ":" + portfolioID.String()
}

func New(db *gorm.DB, router *marketsource.Router) *PriceScheduler {
	s := &PriceScheduler{
		db:     db,
		router: router,
		states: make(map[string]*syncState),
		stopCh: make(chan struct{}),
	}
	s.Start()
	return s
}

func (s *PriceScheduler) Start() {
	s.mu.Lock()
	if s.stopped {
		s.stopped = false
		s.stopCh = make(chan struct{})
	}
	s.mu.Unlock()

	slog.Info("scheduler starting")
	s.loadAndScheduleAll()
}

func (s *PriceScheduler) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	close(s.stopCh)
	for key, state := range s.states {
		state.mu.Lock()
		if state.timer != nil {
			state.timer.Stop()
			state.timer = nil
		}
		state.mu.Unlock()
		delete(s.states, key)
	}
	s.mu.Unlock()
	slog.Info("scheduler stopped")
}

func (s *PriceScheduler) loadAndScheduleAll() {
	var portfolios []models.Portfolio
	if err := s.db.Find(&portfolios).Error; err != nil {
		slog.Error("failed to query portfolios", "error", err)
		return
	}

	for _, p := range portfolios {
		s.schedulePortfolio(p.UserID, p.ID)
	}
}

func (s *PriceScheduler) schedulePortfolio(userID, portfolioID uuid.UUID) {
	var setting models.Setting
	if err := s.db.Where("`key` = ? AND portfolio_id = ?", "syncInterval", portfolioID).First(&setting).Error; err != nil {
		return
	}

	interval, err := strconv.Atoi(setting.Value)
	if err != nil || interval <= 0 {
		s.stopSchedule(userID, portfolioID)
		return
	}

	key := syncKey(userID, portfolioID)
	s.mu.Lock()
	state, exists := s.states[key]
	if !exists {
		state = &syncState{
			interval: time.Duration(interval) * time.Minute,
		}
		s.states[key] = state
	} else {
		state.mu.Lock()
		state.interval = time.Duration(interval) * time.Minute
		state.mu.Unlock()
	}
	s.mu.Unlock()

	state.mu.Lock()
	if state.timer != nil {
		state.timer.Stop()
	}

	state.scheduleVersion++
	version := state.scheduleVersion

	var delay time.Duration
	if state.lastSyncAt.IsZero() {
		delay = 0
	} else {
		elapsed := time.Since(state.lastSyncAt)
		if elapsed >= state.interval {
			delay = 0
		} else {
			delay = state.interval - elapsed
		}
	}

	state.timer = time.AfterFunc(delay, func() {
		s.syncAndReschedule(userID, portfolioID, state, version)
	})
	state.mu.Unlock()

	slog.Info("scheduled portfolio sync", "userId", userID.String(), "portfolioId", portfolioID.String(), "delay", delay)
}

func (s *PriceScheduler) stopSchedule(userID, portfolioID uuid.UUID) {
	key := syncKey(userID, portfolioID)
	s.mu.Lock()
	state, exists := s.states[key]
	if exists {
		state.mu.Lock()
		if state.timer != nil {
			state.timer.Stop()
			state.timer = nil
		}
		state.mu.Unlock()
		delete(s.states, key)
	}
	s.mu.Unlock()

	if exists {
		slog.Info("stopped portfolio sync", "userId", userID.String(), "portfolioId", portfolioID.String())
	}
}

func (s *PriceScheduler) syncAndReschedule(userID, portfolioID uuid.UUID, state *syncState, version uint64) {
	s.mu.RLock()
	stopped := s.stopped
	s.mu.RUnlock()
	if stopped {
		return
	}

	s.syncPortfolio(userID, portfolioID, state)

	state.mu.Lock()
	if state.scheduleVersion != version {
		state.mu.Unlock()
		return
	}
	if state.timer != nil && !s.isStopped() {
		state.timer = time.AfterFunc(state.interval, func() {
			s.syncAndReschedule(userID, portfolioID, state, version)
		})
	}
	state.mu.Unlock()
}

func (s *PriceScheduler) isStopped() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stopped
}

type syncResult struct {
	holding *models.Holding
	result  *marketsource.Quote
	err     error
}

const (
	maxConcurrentFetch = 5
	fetchRateLimit     = 50 * time.Millisecond
)

func (s *PriceScheduler) syncPortfolio(userID, portfolioID uuid.UUID, state *syncState) {
	state.mu.Lock()
	if state.syncing {
		state.mu.Unlock()
		return
	}
	state.syncing = true
	state.mu.Unlock()

	s.publishEvent(userID, portfolioID, EventSyncStarted, SyncStartedData{})

	defer func() {
		state.mu.Lock()
		state.syncing = false
		state.mu.Unlock()
	}()

	slog.Info("starting price sync", "userId", userID.String(), "portfolioId", portfolioID.String())

	var holdings []models.Holding
	if err := s.db.Where("portfolio_id = ? AND symbol != ''", portfolioID).Find(&holdings).Error; err != nil {
		state.mu.Lock()
		state.lastSyncErr = err.Error()
		state.mu.Unlock()
		slog.Error("failed to query holdings", "userId", userID.String(), "portfolioId", portfolioID.String(), "error", err)
		s.publishEvent(userID, portfolioID, EventSyncFailed, SyncFailedData{Error: err.Error()})
		return
	}

	if len(holdings) == 0 {
		slog.Info("no holdings with symbols to sync", "userId", userID.String(), "portfolioId", portfolioID.String())
		state.mu.Lock()
		state.lastSyncAt = time.Now()
		state.lastSyncErr = ""
		lastSyncAt := state.lastSyncAt
		state.mu.Unlock()
		s.publishEvent(userID, portfolioID, EventSyncCompleted, SyncCompletedData{
			LastSyncAt:  lastSyncAt,
			SyncedCount: 0,
			FailedCount: 0,
		})
		return
	}

	synced := 0
	failed := 0
	syncedPrices := make(map[string]decimal.Decimal)

	sem := make(chan struct{}, maxConcurrentFetch)
	results := make(chan syncResult, len(holdings))

	var wg sync.WaitGroup
	for i := range holdings {
		wg.Add(1)
		sem <- struct{}{}
		go func(h *models.Holding) {
			defer func() {
				<-sem
				wg.Done()
			}()
			result, err := s.router.FetchQuote(userID, h.Symbol, h.Market)
			if err != nil {
				results <- syncResult{holding: h, result: nil, err: err}
			} else {
				results <- syncResult{holding: h, result: result, err: nil}
			}
		}(&holdings[i])
		time.Sleep(fetchRateLimit)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.err != nil {
			slog.Error("failed to fetch price", "userId", userID.String(), "portfolioId", portfolioID.String(), "name", r.holding.Name, "symbol", r.holding.Symbol, "error", r.err)
			failed++
			continue
		}

		updates := map[string]any{
			"price": r.result.Price,
			"value": r.holding.Shares.Mul(r.result.Price),
		}
		if err := s.db.Model(&models.Holding{}).Where("id = ? AND portfolio_id = ?", r.holding.ID, portfolioID).Updates(updates).Error; err != nil {
			slog.Error("failed to update holding", "userId", userID.String(), "portfolioId", portfolioID.String(), "id", r.holding.ID, "error", err)
			failed++
			continue
		}

		synced++
		syncedPrices[r.holding.Symbol] = r.result.Price
		slog.Info("synced holding", "userId", userID.String(), "portfolioId", portfolioID.String(), "name", r.holding.Name, "symbol", r.holding.Symbol, "price", r.result.Price)
	}

	state.mu.Lock()
	state.lastSyncAt = time.Now()
	if failed > 0 {
		state.lastSyncErr = fmt.Sprintf("%d/%d failed", failed, synced+failed)
	} else {
		state.lastSyncErr = ""
	}
	lastSyncAt := state.lastSyncAt
	lastSyncErr := state.lastSyncErr
	state.mu.Unlock()

	slog.Info("sync completed", "userId", userID.String(), "portfolioId", portfolioID.String(), "synced", synced, "failed", failed)

	if lastSyncErr != "" {
		s.publishEvent(userID, portfolioID, EventSyncFailed, SyncFailedData{Error: lastSyncErr})
	} else {
		s.publishEvent(userID, portfolioID, EventSyncCompleted, SyncCompletedData{
			LastSyncAt:  lastSyncAt,
			SyncedCount: synced,
			FailedCount: failed,
		})
	}

	var updates []HoldingUpdate
	for symbol, price := range syncedPrices {
		for i := range holdings {
			if holdings[i].Symbol == symbol {
				updates = append(updates, HoldingUpdate{
					Symbol: symbol,
					Price:  price.String(),
					Value:  holdings[i].Shares.Mul(price).String(),
				})
				break
			}
		}
	}
	if len(updates) > 0 {
		s.publishEvent(userID, portfolioID, EventPriceUpdated, PriceUpdatedData{Holdings: updates})
	}

	if s.notifier != nil {
		s.notifier.NotifyAfterSync(userID, portfolioID, holdings, syncedPrices)
	}
}

func (s *PriceScheduler) TriggerSyncForPortfolio(userID, portfolioID uuid.UUID) bool {
	key := syncKey(userID, portfolioID)
	s.mu.Lock()
	state, exists := s.states[key]
	if !exists {
		state = &syncState{}
		s.states[key] = state
	}
	s.mu.Unlock()

	s.router.ClearAllCaches()
	go s.syncPortfolio(userID, portfolioID, state)
	return true
}

func (s *PriceScheduler) TriggerSyncForPortfolioSync(userID, portfolioID uuid.UUID) (SyncStatus, bool) {
	key := syncKey(userID, portfolioID)
	s.mu.Lock()
	state, exists := s.states[key]
	if !exists {
		state = &syncState{}
		s.states[key] = state
	}
	s.mu.Unlock()

	s.router.ClearAllCaches()
	s.syncPortfolio(userID, portfolioID, state)

	state.mu.Lock()
	defer state.mu.Unlock()
	return SyncStatus{
		LastSyncAt:  state.lastSyncAt,
		LastSyncErr: state.lastSyncErr,
		Syncing:     false,
	}, true
}

// UpdateSchedule reschedules sync for a portfolio (called when settings change)
func (s *PriceScheduler) UpdateSchedule(userID, portfolioID uuid.UUID) {
	s.schedulePortfolio(userID, portfolioID)
}

func (s *PriceScheduler) SetNotifier(n *Notifier) {
	s.notifier = n
}

func (s *PriceScheduler) SetEventBus(eb *EventBus) {
	s.eventBus = eb
}

func (s *PriceScheduler) publishEvent(userID, portfolioID uuid.UUID, eventType string, data any) {
	if s.eventBus == nil {
		return
	}
	s.eventBus.Publish(userID, Event{
		Type:        eventType,
		PortfolioID: portfolioID.String(),
		Data:        data,
		Timestamp:   time.Now(),
	})
}
