package scheduler

import (
	"fmt"
	"log/slog"
	"maps"
	"portfolio-management/marketsource"
	"portfolio-management/models"
	"portfolio-management/telegram"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type cachedTelegram struct {
	client *telegram.Client
	token  string
	chatID string
}

type Notifier struct {
	db              *gorm.DB
	router          *marketsource.Router
	mu              sync.RWMutex
	prevPrices      map[string]map[string]decimal.Decimal
	lastDriftAlert  map[string]time.Time
	lastSummaryTime map[string]time.Time
	telegramClients map[string]*cachedTelegram
}

func NewNotifier(db *gorm.DB, router *marketsource.Router) *Notifier {
	return &Notifier{
		db:              db,
		router:          router,
		prevPrices:      make(map[string]map[string]decimal.Decimal),
		lastDriftAlert:  make(map[string]time.Time),
		lastSummaryTime: make(map[string]time.Time),
		telegramClients: make(map[string]*cachedTelegram),
	}
}

func (n *Notifier) loadPortfolioSettings(portfolioID string) map[string]string {
	var settings []models.Setting
	if err := n.db.Where("portfolio_id = ?", portfolioID).Find(&settings).Error; err != nil {
		slog.Error("failed to load portfolio settings", "portfolioId", portfolioID, "error", err)
	}
	result := make(map[string]string, len(settings))
	for i := range settings {
		result[settings[i].Key] = settings[i].Value
	}
	return result
}

func (n *Notifier) loadUserTelegramConfig(userID string) (token, chatID, enabled string) {
	var settings []models.Setting
	if err := n.db.Where("user_id = ? AND `key` IN ('telegramBotToken', 'telegramChatID', 'telegramEnabled')", userID).Find(&settings).Error; err != nil {
		slog.Error("failed to load user telegram config", "userId", userID, "error", err)
	}
	for _, s := range settings {
		switch s.Key {
		case "telegramBotToken":
			token = s.Value
		case "telegramChatID":
			chatID = s.Value
		case "telegramEnabled":
			enabled = s.Value
		}
	}
	return
}

func (n *Notifier) LoadTelegramConfig(userID string) (*telegram.Client, error) {
	token, chatID, enabled := n.loadUserTelegramConfig(userID)

	if enabled != "true" || token == "" || chatID == "" {
		n.mu.Lock()
		delete(n.telegramClients, userID)
		n.mu.Unlock()
		return nil, nil
	}

	n.mu.RLock()
	cached, ok := n.telegramClients[userID]
	n.mu.RUnlock()
	if ok && cached.token == token && cached.chatID == chatID {
		return cached.client, nil
	}

	client, err := telegram.NewClient(token, chatID)
	if err != nil {
		return nil, err
	}

	n.mu.Lock()
	n.telegramClients[userID] = &cachedTelegram{client: client, token: token, chatID: chatID}
	n.mu.Unlock()

	return client, nil
}

func (n *Notifier) NotifyAfterSync(userID, portfolioID string, holdings []models.Holding, syncedSymbols map[string]decimal.Decimal) {
	client, err := n.LoadTelegramConfig(userID)
	if err != nil {
		slog.Error("failed to load telegram config for notification", "userId", userID, "error", err)
		return
	}
	if client == nil {
		return
	}

	n.checkPriceAlert(userID, portfolioID, client, holdings, syncedSymbols)
	n.checkDriftAlert(userID, portfolioID, client)
	n.checkSummary(userID, portfolioID, client, holdings)
}

func (n *Notifier) checkPriceAlert(userID, portfolioID string, client *telegram.Client, holdings []models.Holding, syncedPrices map[string]decimal.Decimal) {
	settings := n.loadPortfolioSettings(portfolioID)

	if settings["telegramPriceAlert"] != "true" {
		return
	}

	threshold := decimal.NewFromInt(5)
	if v := settings["telegramPriceThreshold"]; v != "" {
		if t, err := decimal.NewFromString(v); err == nil {
			threshold = t
		}
	}

	cacheKey := syncKey(userID, portfolioID)
	n.mu.Lock()
	if n.prevPrices[cacheKey] == nil {
		n.prevPrices[cacheKey] = make(map[string]decimal.Decimal)
	}
	oldPrices := n.prevPrices[cacheKey]

	var alerts []string
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
					alerts = append(alerts, fmt.Sprintf(
						"%s <b>%s</b> (%s)\n当前价: ¥%s | 涨跌: %s%%",
						arrow, h.Name, h.Symbol,
						newPrice.StringFixed(2), changePct.StringFixed(1),
					))
					break
				}
			}
		}
	}

	maps.Copy(oldPrices, syncedPrices)
	n.mu.Unlock()

	if len(alerts) > 0 {
		msg := "⚠️ <b>价格波动提醒</b>\n\n" + strings.Join(alerts, "\n\n")
		if err := client.SendMessage(msg); err != nil {
			slog.Error("failed to send price alert", "userId", userID, "portfolioId", portfolioID, "error", err)
		} else {
			slog.Info("sent price alert", "userId", userID, "portfolioId", portfolioID, "count", len(alerts))
		}
	}
}

func (n *Notifier) checkDriftAlert(userID, portfolioID string, client *telegram.Client) {
	settings := n.loadPortfolioSettings(portfolioID)

	if settings["telegramDriftAlert"] != "true" {
		return
	}

	cacheKey := syncKey(userID, portfolioID)
	n.mu.RLock()
	lastAlert, exists := n.lastDriftAlert[cacheKey]
	n.mu.RUnlock()

	if exists && time.Since(lastAlert) < 24*time.Hour {
		return
	}

	driftThreshold := decimal.NewFromInt(5)
	if v := settings["driftThreshold"]; v != "" {
		if t, err := decimal.NewFromString(v); err == nil {
			driftThreshold = t
		}
	}

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

	var alerts []string
	assetNames := map[string]string{
		"stocks":      "股票",
		"bonds":       "债券",
		"cash":        "现金",
		"commodities": "商品",
	}

	for id, value := range assets {
		pct := value.Div(total).Mul(decimal.NewFromInt(100))
		diff := pct.Sub(targetPcts[id])
		if diff.GreaterThan(driftThreshold) || diff.LessThan(driftThreshold.Neg()) {
			alerts = append(alerts, fmt.Sprintf(
				"<b>%s</b>: %s%% (目标 %s%%, 偏离 %s%%)",
				assetNames[id], pct.StringFixed(1), targetPcts[id].StringFixed(0), diff.StringFixed(1),
			))
		}
	}

	if len(alerts) > 0 {
		msg := "⚠️ <b>配比偏离提醒</b>\n\n当前资产配置:\n" + strings.Join(alerts, "\n")
		if err := client.SendMessage(msg); err != nil {
			slog.Error("failed to send drift alert", "userId", userID, "portfolioId", portfolioID, "error", err)
		} else {
			n.mu.Lock()
			n.lastDriftAlert[cacheKey] = time.Now()
			n.mu.Unlock()
			slog.Info("sent drift alert", "userId", userID, "portfolioId", portfolioID, "count", len(alerts))
		}
	}
}

func (n *Notifier) checkSummary(userID, portfolioID string, client *telegram.Client, holdings []models.Holding) {
	settings := n.loadPortfolioSettings(portfolioID)

	if settings["telegramSummary"] != "true" {
		return
	}

	interval := settings["telegramSummaryInterval"]

	shouldSend := false
	now := time.Now()

	cacheKey := syncKey(userID, portfolioID)
	n.mu.RLock()
	lastTime, exists := n.lastSummaryTime[cacheKey]
	n.mu.RUnlock()

	switch interval {
	case "daily":
		if !exists || now.Sub(lastTime) >= 24*time.Hour {
			shouldSend = true
		}
	case "weekly":
		if !exists || now.Sub(lastTime) >= 7*24*time.Hour {
			shouldSend = true
		}
	default:
		return
	}

	if !shouldSend {
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

	lines := []string{
		fmt.Sprintf("📊 <b>投资组合摘要</b> — %s", now.Format("2006-01-02")),
		"",
		fmt.Sprintf("总资产: ¥%s", total.StringFixed(0)),
		fmt.Sprintf("累计投入: ¥%s", principal.StringFixed(0)),
	}
	if principal.IsPositive() {
		pnl := total.Sub(principal).Div(principal).Mul(decimal.NewFromInt(100))
		lines = append(lines, fmt.Sprintf("累计收益: %s%%", pnl.StringFixed(1)))
	}
	lines = append(lines, "")

	for _, id := range []string{"stocks", "bonds", "cash", "commodities"} {
		pct := decimal.Zero
		if total.IsPositive() {
			pct = assets[id].Div(total).Mul(decimal.NewFromInt(100))
		}
		lines = append(lines, fmt.Sprintf("%s  %s%%  ¥%s", assetNames[id], pct.StringFixed(1), assets[id].StringFixed(0)))
	}

	if err := client.SendMessage(strings.Join(lines, "\n")); err != nil {
		slog.Error("failed to send portfolio summary", "userId", userID, "portfolioId", portfolioID, "error", err)
	} else {
		n.mu.Lock()
		n.lastSummaryTime[cacheKey] = now
		n.mu.Unlock()
		slog.Info("sent portfolio summary", "userId", userID, "portfolioId", portfolioID)
	}
}
