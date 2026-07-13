package scheduler

import (
	"log/slog"
	"maps"
	"portfolio-management/internal/marketsource"
	"portfolio-management/internal/notifications"
	"portfolio-management/internal/notifications/bark"
	"portfolio-management/internal/notifications/telegram"
	"portfolio-management/models"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Notifier struct {
	db              *gorm.DB
	router          *marketsource.Router
	mu              sync.RWMutex
	prevPrices      map[string]map[string]decimal.Decimal
	lastDriftAlert  map[string]time.Time
	lastSummaryTime map[string]time.Time
	channels        []notifications.Channel
}

func NewNotifier(db *gorm.DB, router *marketsource.Router) *Notifier {
	n := &Notifier{
		db:              db,
		router:          router,
		prevPrices:      make(map[string]map[string]decimal.Decimal),
		lastDriftAlert:  make(map[string]time.Time),
		lastSummaryTime: make(map[string]time.Time),
	}
	n.channels = []notifications.Channel{
		telegram.NewChannel(db),
		bark.NewChannel(db),
	}
	return n
}

func (n *Notifier) loadPortfolioName(portfolioID uuid.UUID) string {
	var portfolio models.Portfolio
	if err := n.db.Where("id = ?", portfolioID).First(&portfolio).Error; err != nil {
		slog.Error("failed to load portfolio name", "portfolioId", portfolioID, "error", err)
		return ""
	}
	return portfolio.Name
}

func (n *Notifier) NotifyAfterSync(userID, portfolioID uuid.UUID, holdings []models.Holding, syncedSymbols map[string]decimal.Decimal) {
	var activeChannels []notifications.Channel
	for _, ch := range n.channels {
		if ch.IsEnabled(userID, portfolioID, n.db) {
			activeChannels = append(activeChannels, ch)
		}
	}
	if len(activeChannels) == 0 {
		return
	}

	portfolioName := n.loadPortfolioName(portfolioID)
	n.checkPriceAlert(userID, portfolioID, portfolioName, activeChannels, holdings, syncedSymbols)
	n.checkDriftAlert(userID, portfolioID, portfolioName, activeChannels)
	n.checkSummary(userID, portfolioID, portfolioName, activeChannels, holdings)
}

func (n *Notifier) checkPriceAlert(userID, portfolioID uuid.UUID, portfolioName string, channels []notifications.Channel, holdings []models.Holding, syncedPrices map[string]decimal.Decimal) {
	settings := notifications.LoadPortfolioSettings(n.db, portfolioID)
	threshold := notifications.ParseThreshold(settings, "telegramPriceThreshold", "barkPriceThreshold")

	key := notifications.StateKey(userID, portfolioID)
	n.mu.Lock()
	if n.prevPrices[key] == nil {
		n.prevPrices[key] = make(map[string]decimal.Decimal)
	}
	oldPrices := n.prevPrices[key]

	var alerts []notifications.PriceAlert
	for symbol, newPrice := range syncedPrices {
		oldPrice, ok := oldPrices[symbol]
		if !ok || oldPrice.IsZero() {
			continue
		}

		changePct := newPrice.Sub(oldPrice).Div(oldPrice).Mul(decimal.NewFromInt(100))
		if changePct.GreaterThan(threshold) || changePct.LessThan(threshold.Neg()) {
			for i := range holdings {
				h := &holdings[i]
				if h.Symbol == symbol {
					arrow := "📈"
					if changePct.IsNegative() {
						arrow = "📉"
					}
					alerts = append(alerts, notifications.PriceAlert{
						Arrow:     arrow,
						Name:      h.Name,
						Symbol:    h.Symbol,
						Price:     newPrice.StringFixed(2),
						ChangePct: changePct.StringFixed(1),
					})
					break
				}
			}
		}
	}

	maps.Copy(oldPrices, syncedPrices)
	n.mu.Unlock()

	if len(alerts) == 0 {
		return
	}

	for _, ch := range channels {
		if err := ch.SendPriceAlert(userID, portfolioID, portfolioName, alerts, n.db); err != nil {
			slog.Error("failed to send price alert", "channel", ch.Name(), "userId", userID, "portfolioId", portfolioID, "error", err)
		}
	}
}

func (n *Notifier) checkDriftAlert(userID, portfolioID uuid.UUID, portfolioName string, channels []notifications.Channel) {
	settings := notifications.LoadPortfolioSettings(n.db, portfolioID)

	key := notifications.StateKey(userID, portfolioID)
	n.mu.RLock()
	lastAlert, exists := n.lastDriftAlert[key]
	n.mu.RUnlock()

	if exists && time.Since(lastAlert) < 24*time.Hour {
		return
	}

	driftThreshold := notifications.ParseThreshold(settings, "driftThreshold")

	var holdings []models.Holding
	if err := n.db.Where("portfolio_id = ?", portfolioID).Find(&holdings).Error; err != nil {
		return
	}

	for i := range holdings {
		h := &holdings[i]
		if h.Currency != "" && h.Currency != "CNY" {
			pair := h.Currency + "CNY"
			rate, err := n.router.ExchangeRate(userID, pair)
			if err == nil {
				h.Value = h.Value.Mul(rate)
			}
		}
	}

	assets := map[string]decimal.Decimal{
		"stocks":      decimal.Zero,
		"bonds":       decimal.Zero,
		"cash":        decimal.Zero,
		"commodities": decimal.Zero,
	}
	total := decimal.Zero
	for i := range holdings {
		h := &holdings[i]
		assets[h.AssetId] = assets[h.AssetId].Add(h.Value)
		total = total.Add(h.Value)
	}

	var funds []models.AvailableFund
	if err := n.db.Where("user_id = ? AND portfolio_id = ?", userID, portfolioID).Find(&funds).Error; err != nil {
		slog.Error("failed to load available funds for drift alert", "userId", userID, "portfolioId", portfolioID, "error", err)
	}
	for _, f := range funds {
		amt := f.Amount
		if f.Currency != "" && f.Currency != "CNY" {
			pair := f.Currency + "CNY"
			rate, err := n.router.ExchangeRate(userID, pair)
			if err == nil {
				amt = amt.Mul(rate)
			}
		}
		total = total.Add(amt)
	}

	if total.IsZero() {
		return
	}

	targetPcts := map[string]decimal.Decimal{
		"stocks":      decimal.NewFromInt(25),
		"bonds":       decimal.NewFromInt(25),
		"cash":        decimal.NewFromInt(25),
		"commodities": decimal.NewFromInt(25),
	}
	for id := range targetPcts {
		if v := settings["target"+strings.ToUpper(id[:1])+id[1:]]; v != "" {
			if pct, err := decimal.NewFromString(v); err == nil {
				targetPcts[id] = pct
			}
		}
	}

	targetTotal := decimal.Zero
	for _, v := range targetPcts {
		targetTotal = targetTotal.Add(v)
	}
	if targetTotal.GreaterThan(decimal.Zero) && !targetTotal.Equal(decimal.NewFromInt(100)) {
		for id := range targetPcts {
			targetPcts[id] = targetPcts[id].Div(targetTotal).Mul(decimal.NewFromInt(100))
		}
	}

	assetNames := map[string]string{
		"stocks":      "股票",
		"bonds":       "债券",
		"cash":        "现金",
		"commodities": "商品",
	}

	var alerts []notifications.DriftAlert
	for id, value := range assets {
		pct := value.Div(total).Mul(decimal.NewFromInt(100))
		diff := pct.Sub(targetPcts[id])
		if diff.GreaterThan(driftThreshold) || diff.LessThan(driftThreshold.Neg()) {
			alerts = append(alerts, notifications.DriftAlert{
				AssetName: assetNames[id],
				Pct:       pct.StringFixed(1),
				Target:    targetPcts[id].StringFixed(0),
				Diff:      diff.StringFixed(1),
			})
		}
	}

	if len(alerts) == 0 {
		return
	}

	sent := false
	for _, ch := range channels {
		if err := ch.SendDriftAlert(userID, portfolioID, portfolioName, alerts, n.db); err != nil {
			slog.Error("failed to send drift alert", "channel", ch.Name(), "userId", userID, "portfolioId", portfolioID, "error", err)
		} else {
			sent = true
		}
	}

	if sent {
		n.mu.Lock()
		n.lastDriftAlert[key] = time.Now()
		n.mu.Unlock()
	}
}

func (n *Notifier) checkSummary(userID, portfolioID uuid.UUID, portfolioName string, channels []notifications.Channel, holdings []models.Holding) {
	now := time.Now()
	key := notifications.StateKey(userID, portfolioID)

	n.mu.RLock()
	lastTime := n.lastSummaryTime[key]
	n.mu.RUnlock()

	var shouldSendChannels []notifications.Channel
	for _, ch := range channels {
		if ch.ShouldSendSummary(userID, portfolioID, lastTime, now, n.db) {
			shouldSendChannels = append(shouldSendChannels, ch)
		}
	}
	if len(shouldSendChannels) == 0 {
		return
	}

	for i := range holdings {
		h := &holdings[i]
		if h.Currency != "" && h.Currency != "CNY" {
			pair := h.Currency + "CNY"
			rate, err := n.router.ExchangeRate(userID, pair)
			if err != nil {
				slog.Error("failed to fetch exchange rate for summary", "pair", pair, "error", err)
				continue
			}
			h.Value = h.Value.Mul(rate)
			h.Cost = h.Cost.Mul(rate)
		}
	}

	assets := map[string]decimal.Decimal{
		"stocks":      decimal.Zero,
		"bonds":       decimal.Zero,
		"cash":        decimal.Zero,
		"commodities": decimal.Zero,
	}
	total := decimal.Zero
	for i := range holdings {
		h := &holdings[i]
		assets[h.AssetId] = assets[h.AssetId].Add(h.Value)
		total = total.Add(h.Value)
	}

	var funds []models.AvailableFund
	if err := n.db.Where("user_id = ? AND portfolio_id = ?", userID, portfolioID).Find(&funds).Error; err != nil {
		slog.Error("failed to load available funds for summary", "userId", userID, "portfolioId", portfolioID, "error", err)
	}
	for _, f := range funds {
		amt := f.Amount
		if f.Currency != "" && f.Currency != "CNY" {
			pair := f.Currency + "CNY"
			rate, err := n.router.ExchangeRate(userID, pair)
			if err == nil {
				amt = amt.Mul(rate)
			}
		}
		total = total.Add(amt)
	}

	var txs []models.FundTransaction
	if err := n.db.Where("portfolio_id = ? AND type IN ?", portfolioID, []string{"transfer_in", "transfer_out"}).Find(&txs).Error; err != nil {
		slog.Error("failed to load fund transactions for summary", "portfolioId", portfolioID, "error", err)
	}
	byCurrency := make(map[string]decimal.Decimal)
	for i := range txs {
		if txs[i].Type == "transfer_in" {
			byCurrency[txs[i].Currency] = byCurrency[txs[i].Currency].Add(txs[i].Amount)
		} else {
			byCurrency[txs[i].Currency] = byCurrency[txs[i].Currency].Sub(txs[i].Amount)
		}
	}
	principal := decimal.Zero
	for currency, amount := range byCurrency {
		if currency == "CNY" || amount.IsZero() {
			principal = principal.Add(amount)
			continue
		}
		rate, err := n.router.ExchangeRate(userID, currency+"CNY")
		if err == nil {
			principal = principal.Add(amount.Mul(rate))
		}
	}

	assetNames := map[string]string{
		"stocks":      "股票",
		"bonds":       "债券",
		"cash":        "现金",
		"commodities": "商品",
	}

	// Group holdings by asset class
	holdingsByAsset := make(map[string][]models.Holding)
	for i := range holdings {
		h := &holdings[i]
		holdingsByAsset[h.AssetId] = append(holdingsByAsset[h.AssetId], *h)
	}

	summaryAssets := make([]notifications.SummaryAsset, 0, 4)
	for _, id := range []string{"stocks", "bonds", "cash", "commodities"} {
		pct := decimal.Zero
		if total.IsPositive() {
			pct = assets[id].Div(total).Mul(decimal.NewFromInt(100))
		}

		var summaryHoldings []notifications.SummaryHolding
		for _, h := range holdingsByAsset[id] {
			hPct := decimal.Zero
			if total.IsPositive() {
				hPct = h.Value.Div(total).Mul(decimal.NewFromInt(100))
			}
			hPnL := ""
			if h.Cost.IsPositive() {
				pnl := h.Value.Sub(h.Cost).Div(h.Cost).Mul(decimal.NewFromInt(100))
				hPnL = pnl.StringFixed(1)
			}
			summaryHoldings = append(summaryHoldings, notifications.SummaryHolding{
				Name:  h.Name,
				Pct:   hPct.StringFixed(1),
				Value: h.Value.StringFixed(2),
				PnL:   hPnL,
			})
		}

		summaryAssets = append(summaryAssets, notifications.SummaryAsset{
			Name:     assetNames[id],
			Pct:      pct.StringFixed(1),
			Value:    assets[id].StringFixed(2),
			Holdings: summaryHoldings,
		})
	}

	data := notifications.SummaryData{
		Total:     total.StringFixed(2),
		Principal: principal.StringFixed(2),
		Date:      now.Format("2006-01-02"),
		Assets:    summaryAssets,
	}
	if principal.IsPositive() {
		pnl := total.Sub(principal).Div(principal).Mul(decimal.NewFromInt(100))
		data.PnL = pnl.StringFixed(2)
	}

	sent := false
	for _, ch := range shouldSendChannels {
		if err := ch.SendSummary(userID, portfolioID, portfolioName, data, n.db); err != nil {
			slog.Error("failed to send summary", "channel", ch.Name(), "userId", userID, "portfolioId", portfolioID, "error", err)
		} else {
			sent = true
		}
	}

	if sent {
		n.mu.Lock()
		n.lastSummaryTime[key] = now
		n.mu.Unlock()
	}
}
