package marketsource

type MarketSource interface {
	Name() string
	SupportedMarkets() []string
	FetchQuote(symbol string, market string) (*Quote, error)
	FetchExchangeRate(pair string) (float64, error)
}
