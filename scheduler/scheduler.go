package scheduler

import (
	"fmt"
	"log/slog"
	"portfolio-management/eastmoney"
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
	states   map[string]*syncState
	notifier *Notifier
}

type syncState struct {
	interval    time.Duration
	lastSyncAt  time.Time
	lastSyncErr string
	syncing     bool
	mu          sync.Mutex
}

func syncKey(userID, portfolioID string) string {
	return userID + ":" + portfolioID
}

func New(db *gorm.DB) *PriceScheduler {
	s := &PriceScheduler{
		db:     db,
		stopCh: make(chan struct{}),
		states: make(map[string]*syncState),
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
			s.syncAllPortfolios()
		case <-stopCh:
			return
		}
	}
}

func (s *PriceScheduler) syncAllPortfolios() {
	var portfolios []models.Portfolio
	if err := s.db.Find(&portfolios).Error; err != nil {
		slog.Error("failed to query portfolios", "error", err)
		return
	}

	for _, p := range portfolios {
		var setting models.Setting
		if err := s.db.Where("`key` = ? AND portfolio_id = ?", "syncInterval", p.ID).First(&setting).Error; err != nil {
			continue
		}

		interval, err := strconv.Atoi(setting.Value)
		if err != nil || interval <= 0 {
			continue
		}

		key := syncKey(p.UserID, p.ID)
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
		shouldSync := time.Since(state.lastSyncAt) >= state.interval && !state.syncing
		state.mu.Unlock()

		if shouldSync {
			go s.syncPortfolio(p.UserID, p.ID, state)
		}
	}
}

type syncResult struct {
	holding *models.Holding
	result  *yahoo.PriceResult
	err     error
}

const (
	maxConcurrentFetch = 5
	fetchRateLimit     = 50 * time.Millisecond
)

func (s *PriceScheduler) syncPortfolio(userID, portfolioID string, state *syncState) {
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

	slog.Info("starting price sync", "userId", userID, "portfolioId", portfolioID)

	var holdings []models.Holding
	if err := s.db.Where("portfolio_id = ? AND symbol != ''", portfolioID).Find(&holdings).Error; err != nil {
		state.mu.Lock()
		state.lastSyncErr = err.Error()
		state.mu.Unlock()
		slog.Error("failed to query holdings", "userId", userID, "portfolioId", portfolioID, "error", err)
		return
	}

	if len(holdings) == 0 {
		slog.Info("no holdings with symbols to sync", "userId", userID, "portfolioId", portfolioID)
		state.mu.Lock()
		state.lastSyncAt = time.Now()
		state.lastSyncErr = ""
		state.mu.Unlock()
		return
	}

	synced := 0
	failed := 0
	syncedPrices := make(map[string]float64)

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
			var result *yahoo.PriceResult
			var err error
			switch h.Market {
			case "FUND":
				var r *eastmoney.PriceResult
				r, err = eastmoney.FetchFundQuote(h.Symbol)
				if err == nil {
					result = &yahoo.PriceResult{Symbol: r.Symbol, Name: r.Name, Price: r.Price, Currency: r.Currency}
				}
			case "CN":
				var r *eastmoney.PriceResult
				r, err = eastmoney.FetchAShareQuote(h.Symbol)
				if err == nil {
					result = &yahoo.PriceResult{Symbol: r.Symbol, Name: r.Name, Price: r.Price, Currency: r.Currency}
				}
			default:
				result, err = yahoo.FetchQuote(h.Symbol)
			}
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
			slog.Error("failed to fetch price", "userId", userID, "portfolioId", portfolioID, "name", r.holding.Name, "symbol", r.holding.Symbol, "error", r.err)
			failed++
			continue
		}

		updates := map[string]any{
			"price": r.result.Price,
			"value": gorm.Expr("shares * ?", r.result.Price),
		}
		if err := s.db.Model(&models.Holding{}).Where("id = ? AND portfolio_id = ?", r.holding.ID, portfolioID).Updates(updates).Error; err != nil {
			slog.Error("failed to update holding", "userId", userID, "portfolioId", portfolioID, "id", r.holding.ID, "error", err)
			failed++
			continue
		}

		synced++
		syncedPrices[r.holding.Symbol] = r.result.Price
		slog.Info("synced holding", "userId", userID, "portfolioId", portfolioID, "name", r.holding.Name, "symbol", r.holding.Symbol, "price", r.result.Price)
	}

	state.mu.Lock()
	state.lastSyncAt = time.Now()
	if failed > 0 {
		state.lastSyncErr = fmt.Sprintf("%d/%d failed", failed, synced+failed)
	} else {
		state.lastSyncErr = ""
	}
	state.mu.Unlock()

	slog.Info("sync completed", "userId", userID, "portfolioId", portfolioID, "synced", synced, "failed", failed)

	if s.notifier != nil {
		s.notifier.NotifyAfterSync(userID, portfolioID, holdings, syncedPrices)
	}
}

func (s *PriceScheduler) GetStatusForPortfolio(userID, portfolioID string) SyncStatus {
	key := syncKey(userID, portfolioID)
	s.mu.RLock()
	state, exists := s.states[key]
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

func (s *PriceScheduler) TriggerSyncForPortfolio(userID, portfolioID string) bool {
	key := syncKey(userID, portfolioID)
	s.mu.Lock()
	state, exists := s.states[key]
	if !exists {
		state = &syncState{}
		s.states[key] = state
	}
	s.mu.Unlock()

	yahoo.ClearRateCache()
	eastmoney.ClearCache()
	go s.syncPortfolio(userID, portfolioID, state)
	return true
}

func (s *PriceScheduler) TriggerSyncForPortfolioSync(userID, portfolioID string) (SyncStatus, bool) {
	key := syncKey(userID, portfolioID)
	s.mu.Lock()
	state, exists := s.states[key]
	if !exists {
		state = &syncState{}
		s.states[key] = state
	}
	s.mu.Unlock()

	yahoo.ClearRateCache()
	eastmoney.ClearCache()
	s.syncPortfolio(userID, portfolioID, state)

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
