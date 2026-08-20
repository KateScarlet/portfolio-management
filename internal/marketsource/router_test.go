package marketsource

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/shopspring/decimal"
)

type mockSource struct {
	name          string
	markets       []string
	quoteCalls    atomic.Int32
	exchangeCalls atomic.Int32
}

func (m *mockSource) Name() string               { return m.name }
func (m *mockSource) SupportedMarkets() []string { return m.markets }
func (m *mockSource) FetchQuote(symbol, market string) (*Quote, error) {
	m.quoteCalls.Add(1)
	return &Quote{Symbol: symbol, Price: decimal.NewFromInt(100), Currency: "USD"}, nil
}

func (m *mockSource) FetchExchangeRate(pair string) (decimal.Decimal, error) {
	m.exchangeCalls.Add(1)
	return decimal.NewFromFloat(7.2), nil
}

func newTestRouter(t *testing.T, src *mockSource) *Router {
	t.Helper()
	return NewRouter(nil, map[string]MarketSource{src.name: src})
}

func TestFetchQuote_CachesResult(t *testing.T) {
	src := &mockSource{name: "eastmoney", markets: []string{"US"}}
	r := newTestRouter(t, src)

	q1, err := r.FetchQuote(uuid.UUID{}, "AAPL", "US")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	q2, err := r.FetchQuote(uuid.UUID{}, "AAPL", "US")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if src.quoteCalls.Load() != 1 {
		t.Errorf("expected source called once, got %d", src.quoteCalls.Load())
	}
	if !q1.Price.Equal(q2.Price) {
		t.Errorf("expected same quote, got %v vs %v", q1, q2)
	}
}

func TestExchangeRate_CachesResult(t *testing.T) {
	src := &mockSource{name: "sina", markets: []string{"EXCHANGE"}}
	r := newTestRouter(t, src)

	rate1, err := r.ExchangeRate(uuid.UUID{}, "USD/CNY")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	rate2, err := r.ExchangeRate(uuid.UUID{}, "USD/CNY")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if src.exchangeCalls.Load() != 1 {
		t.Errorf("expected source called once, got %d", src.exchangeCalls.Load())
	}
	if !rate1.Equal(rate2) {
		t.Errorf("expected same rate, got %v vs %v", rate1, rate2)
	}
}

func TestFetchQuote_CacheExpires(t *testing.T) {
	src := &mockSource{name: "eastmoney", markets: []string{"US"}}
	r := newTestRouter(t, src)
	r.cacheTTL = 50 * time.Millisecond

	_, _ = r.FetchQuote(uuid.UUID{}, "AAPL", "US")
	time.Sleep(60 * time.Millisecond)
	_, _ = r.FetchQuote(uuid.UUID{}, "AAPL", "US")

	if src.quoteCalls.Load() != 2 {
		t.Errorf("expected source called twice after expiry, got %d", src.quoteCalls.Load())
	}
}

func TestClearAllCaches(t *testing.T) {
	src := &mockSource{name: "sina", markets: []string{"US", "EXCHANGE"}}
	r := newTestRouter(t, src)

	_, _ = r.FetchQuote(uuid.UUID{}, "AAPL", "US")
	_, _ = r.ExchangeRate(uuid.UUID{}, "USD/CNY")
	r.ClearAllCaches()

	_, _ = r.FetchQuote(uuid.UUID{}, "AAPL", "US")
	_, _ = r.ExchangeRate(uuid.UUID{}, "USD/CNY")

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

	_, _ = r.FetchQuote(uuid.UUID{}, "AAPL", "US")
	_, _ = r.FetchQuote(uuid.UUID{}, "MSFT", "US")
	_, _ = r.FetchQuote(uuid.UUID{}, "AAPL", "US")

	if src.quoteCalls.Load() != 2 {
		t.Errorf("expected 2 calls for 2 distinct symbols, got %d", src.quoteCalls.Load())
	}
}

// Ensure Router still satisfies its usage (compile-time check).
var _ interface {
	FetchQuote(userID uuid.UUID, symbol, market string) (*Quote, error)
	ExchangeRate(userID uuid.UUID, pair string) (decimal.Decimal, error)
	ClearAllCaches()
} = (*Router)(nil)

// --- EXCHANGE market category tests ---

func TestExchangeRate_UsesExchangeMarketCategory(t *testing.T) {
	src := &mockSource{name: "sina", markets: []string{"EXCHANGE"}}
	r := newTestRouter(t, src)

	rate, err := r.ExchangeRate(uuid.UUID{}, "USDCNY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rate.Equal(decimal.NewFromFloat(7.2)) {
		t.Errorf("expected rate 7.2, got %v", rate)
	}
}

func TestExchangeRate_FallbackToNextSource(t *testing.T) {
	src1 := &failingSource{name: "sina", markets: []string{"EXCHANGE"}}
	src2 := &mockSource{name: "yahoo", markets: []string{"EXCHANGE"}}
	r := NewRouter(nil, map[string]MarketSource{
		"sina":  src1,
		"yahoo": src2,
	})

	rate, err := r.ExchangeRate(uuid.UUID{}, "USDCNY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rate.Equal(decimal.NewFromFloat(7.2)) {
		t.Errorf("expected rate 7.2 from fallback source, got %v", rate)
	}
	if src1.exchangeCalls.Load() != 1 {
		t.Errorf("expected first source called once, got %d", src1.exchangeCalls.Load())
	}
	if src2.exchangeCalls.Load() != 1 {
		t.Errorf("expected second source called once, got %d", src2.exchangeCalls.Load())
	}
}

func TestExchangeRate_SkipsNotSupported(t *testing.T) {
	src1 := &notSupportedSource{name: "eastmoney", markets: []string{"EXCHANGE"}}
	src2 := &mockSource{name: "sina", markets: []string{"EXCHANGE"}}
	r := NewRouter(nil, map[string]MarketSource{
		"eastmoney": src1,
		"sina":      src2,
	})

	rate, err := r.ExchangeRate(uuid.UUID{}, "USDCNY")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rate.Equal(decimal.NewFromFloat(7.2)) {
		t.Errorf("expected rate 7.2 from second source, got %v", rate)
	}
}

func TestExchangeRate_AllSourcesFail(t *testing.T) {
	src1 := &failingSource{name: "eastmoney", markets: []string{"EXCHANGE"}}
	src2 := &failingSource{name: "sina", markets: []string{"EXCHANGE"}}
	r := NewRouter(nil, map[string]MarketSource{
		"eastmoney": src1,
		"sina":      src2,
	})

	_, err := r.ExchangeRate(uuid.UUID{}, "USDCNY")
	if err == nil {
		t.Fatal("expected error when all sources fail")
	}
}

func TestExchangeRate_DifferentPairsCachedSeparately(t *testing.T) {
	src := &mockSource{name: "sina", markets: []string{"EXCHANGE"}}
	r := newTestRouter(t, src)

	_, _ = r.ExchangeRate(uuid.UUID{}, "USDCNY")
	_, _ = r.ExchangeRate(uuid.UUID{}, "EURCNY")
	_, _ = r.ExchangeRate(uuid.UUID{}, "USDCNY")

	if src.exchangeCalls.Load() != 2 {
		t.Errorf("expected 2 calls for 2 distinct pairs, got %d", src.exchangeCalls.Load())
	}
}

func TestExchangeRate_CacheExpires(t *testing.T) {
	src := &mockSource{name: "sina", markets: []string{"EXCHANGE"}}
	r := newTestRouter(t, src)
	r.cacheTTL = 50 * time.Millisecond

	_, _ = r.ExchangeRate(uuid.UUID{}, "USDCNY")
	time.Sleep(60 * time.Millisecond)
	_, _ = r.ExchangeRate(uuid.UUID{}, "USDCNY")

	if src.exchangeCalls.Load() != 2 {
		t.Errorf("expected source called twice after expiry, got %d", src.exchangeCalls.Load())
	}
}

func TestExchangeRate_ClearCacheRefetches(t *testing.T) {
	src := &mockSource{name: "sina", markets: []string{"EXCHANGE"}}
	r := newTestRouter(t, src)

	_, _ = r.ExchangeRate(uuid.UUID{}, "USDCNY")
	r.ClearAllCaches()
	_, _ = r.ExchangeRate(uuid.UUID{}, "USDCNY")

	if src.exchangeCalls.Load() != 2 {
		t.Errorf("expected 2 calls after cache clear, got %d", src.exchangeCalls.Load())
	}
}

func TestAvailableSources_IncludesExchange(t *testing.T) {
	src := &mockSource{name: "sina", markets: []string{"US", "CN", "EXCHANGE"}}
	r := newTestRouter(t, src)

	available := r.AvailableSources()
	if _, ok := available["EXCHANGE"]; !ok {
		t.Error("expected EXCHANGE in available sources")
	}
}

// --- Mock sources for testing ---

type failingSource struct {
	name          string
	markets       []string
	exchangeCalls atomic.Int32
}

func (f *failingSource) Name() string               { return f.name }
func (f *failingSource) SupportedMarkets() []string { return f.markets }
func (f *failingSource) FetchQuote(symbol, market string) (*Quote, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *failingSource) FetchExchangeRate(pair string) (decimal.Decimal, error) {
	f.exchangeCalls.Add(1)
	return decimal.Zero, fmt.Errorf("source unavailable")
}

type notSupportedSource struct {
	name          string
	markets       []string
	exchangeCalls atomic.Int32
}

func (n *notSupportedSource) Name() string               { return n.name }
func (n *notSupportedSource) SupportedMarkets() []string { return n.markets }
func (n *notSupportedSource) FetchQuote(symbol, market string) (*Quote, error) {
	return nil, fmt.Errorf("not implemented")
}

func (n *notSupportedSource) FetchExchangeRate(pair string) (decimal.Decimal, error) {
	n.exchangeCalls.Add(1)
	return decimal.Zero, ErrNotSupported
}
