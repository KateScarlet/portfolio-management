package marketsource

import (
	"sync/atomic"
	"testing"
	"time"
)

type mockSource struct {
	name            string
	markets         []string
	quoteCalls      atomic.Int32
	exchangeCalls   atomic.Int32
}

func (m *mockSource) Name() string            { return m.name }
func (m *mockSource) SupportedMarkets() []string { return m.markets }
func (m *mockSource) FetchQuote(symbol, market string) (*Quote, error) {
	m.quoteCalls.Add(1)
	return &Quote{Symbol: symbol, Price: 100.0, Currency: "USD"}, nil
}
func (m *mockSource) FetchExchangeRate(pair string) (float64, error) {
	m.exchangeCalls.Add(1)
	return 7.2, nil
}

func newTestRouter(t *testing.T, src *mockSource) *Router {
	t.Helper()
	return NewRouter(nil, map[string]MarketSource{src.name: src})
}

func TestFetchQuote_CachesResult(t *testing.T) {
	src := &mockSource{name: "eastmoney", markets: []string{"US"}}
	r := newTestRouter(t, src)

	q1, err := r.FetchQuote("", "AAPL", "US")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	q2, err := r.FetchQuote("", "AAPL", "US")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if src.quoteCalls.Load() != 1 {
		t.Errorf("expected source called once, got %d", src.quoteCalls.Load())
	}
	if q1.Price != q2.Price {
		t.Errorf("expected same quote, got %v vs %v", q1, q2)
	}
}

func TestExchangeRate_CachesResult(t *testing.T) {
	src := &mockSource{name: "eastmoney", markets: []string{"US"}}
	r := newTestRouter(t, src)

	rate1, err := r.ExchangeRate("", "USD/CNY")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	rate2, err := r.ExchangeRate("", "USD/CNY")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if src.exchangeCalls.Load() != 1 {
		t.Errorf("expected source called once, got %d", src.exchangeCalls.Load())
	}
	if rate1 != rate2 {
		t.Errorf("expected same rate, got %v vs %v", rate1, rate2)
	}
}

func TestFetchQuote_CacheExpires(t *testing.T) {
	src := &mockSource{name: "eastmoney", markets: []string{"US"}}
	r := newTestRouter(t, src)
	r.cacheTTL = 50 * time.Millisecond

	_, _ = r.FetchQuote("", "AAPL", "US")
	time.Sleep(60 * time.Millisecond)
	_, _ = r.FetchQuote("", "AAPL", "US")

	if src.quoteCalls.Load() != 2 {
		t.Errorf("expected source called twice after expiry, got %d", src.quoteCalls.Load())
	}
}

func TestClearAllCaches(t *testing.T) {
	src := &mockSource{name: "eastmoney", markets: []string{"US"}}
	r := newTestRouter(t, src)

	_, _ = r.FetchQuote("", "AAPL", "US")
	_, _ = r.ExchangeRate("", "USD/CNY")
	r.ClearAllCaches()

	_, _ = r.FetchQuote("", "AAPL", "US")
	_, _ = r.ExchangeRate("", "USD/CNY")

	if src.quoteCalls.Load() != 2 {
		t.Errorf("expected 2 quote calls after clear, got %d", src.quoteCalls.Load())
	}
	if src.exchangeCalls.Load() != 2 {
		t.Errorf("expected 2 exchange calls after clear, got %d", src.exchangeCalls.Load())
	}
}

func TestFetchQuote_DifferentKeysCachedSeparately(t *testing.T) {
	src := &mockSource{name: "eastmoney", markets: []string{"US"}}
	r := newTestRouter(t, src)

	_, _ = r.FetchQuote("", "AAPL", "US")
	_, _ = r.FetchQuote("", "MSFT", "US")
	_, _ = r.FetchQuote("", "AAPL", "US")

	if src.quoteCalls.Load() != 2 {
		t.Errorf("expected 2 calls for 2 distinct symbols, got %d", src.quoteCalls.Load())
	}
}

// Ensure Router still satisfies its usage (compile-time check).
var _ interface {
	FetchQuote(userID, symbol, market string) (*Quote, error)
	ExchangeRate(userID, pair string) (float64, error)
	ClearAllCaches()
} = (*Router)(nil)
