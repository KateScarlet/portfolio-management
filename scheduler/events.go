package scheduler

import "time"

const (
	EventSyncStarted   = "sync.started"
	EventSyncCompleted = "sync.completed"
	EventSyncFailed    = "sync.failed"
	EventPriceUpdated  = "price.updated"
)

type Event struct {
	Type        string    `json:"type"`
	PortfolioID string    `json:"portfolioId"`
	Data        any       `json:"data"`
	Timestamp   time.Time `json:"timestamp"`
}

type SyncStartedData struct{}

type SyncCompletedData struct {
	LastSyncAt  time.Time `json:"lastSyncAt"`
	SyncedCount int       `json:"syncedCount"`
	FailedCount int       `json:"failedCount"`
}

type SyncFailedData struct {
	Error string `json:"error"`
}

type HoldingUpdate struct {
	Symbol string  `json:"symbol"`
	Price  float64 `json:"price"`
	Value  float64 `json:"value"`
}

type PriceUpdatedData struct {
	Holdings []HoldingUpdate `json:"holdings"`
}
