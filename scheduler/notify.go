package scheduler

import (
	"fmt"
	"log/slog"
	"maps"
	"portfolio-management/models"
	"portfolio-management/telegram"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

type cachedTelegram struct {
	client *telegram.Client
	token  string
	chatID string
}

type Notifier struct {
	db              *gorm.DB
	mu              sync.RWMutex
	prevPrices      map[string]map[string]float64
	lastDriftAlert  map[string]time.Time
	lastSummaryTime map[string]time.Time
	telegramClients map[string]*cachedTelegram
}

func NewNotifier(db *gorm.DB) *Notifier {
	return &Notifier{
		db:              db,
		prevPrices:      make(map[string]map[string]float64),
		lastDriftAlert:  make(map[string]time.Time),
		lastSummaryTime: make(map[string]time.Time),
		telegramClients: make(map[string]*cachedTelegram),
	}
}

func (n *Notifier) loadPortfolioSettings(portfolioID string) map[string]string {
	var settings []models.Setting
	_ = n.db.Where("portfolio_id = ?", portfolioID).Find(&settings).Error
	result := make(map[string]string, len(settings))
	for i := range settings {
		result[settings[i].Key] = settings[i].Value
	}
	return result
}

func (n *Notifier) loadUserTelegramConfig(userID string) (token, chatID, enabled string) {
	var settings []models.Setting
	_ = n.db.Where("user_id = ? AND `key` IN ('telegramBotToken', 'telegramChatID', 'telegramEnabled')", userID).Find(&settings).Error
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

func (n *Notifier) NotifyAfterSync(userID, portfolioID string, holdings []models.Holding, syncedSymbols map[string]float64) {
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

func (n *Notifier) checkPriceAlert(userID, portfolioID string, client *telegram.Client, holdings []models.Holding, syncedPrices map[string]float64) {
	settings := n.loadPortfolioSettings(portfolioID)

	if settings["telegramPriceAlert"] != "true" {
		return
	}

	threshold := 5.0
	if v := settings["telegramPriceThreshold"]; v != "" {
		_, _ = fmt.Sscanf(v, "%f", &threshold)
	}

	cacheKey := syncKey(userID, portfolioID)
	n.mu.Lock()
	if n.prevPrices[cacheKey] == nil {
		n.prevPrices[cacheKey] = make(map[string]float64)
	}
	oldPrices := n.prevPrices[cacheKey]

	var alerts []string
	for symbol, newPrice := range syncedPrices {
		oldPrice, ok := oldPrices[symbol]
		if !ok || oldPrice == 0 {
			continue
		}

		changePct := (newPrice - oldPrice) / oldPrice * 100
		if changePct > threshold || changePct < -threshold {
			for i := range holdings {
				h := &holdings[i]
				if h.Symbol == symbol {
					arrow := "📈"
					if changePct < 0 {
						arrow = "📉"
					}
					alerts = append(alerts, fmt.Sprintf(
						"%s <b>%s</b> (%s)\n当前价: ¥%.2f | 涨跌: %+.1f%%",
						arrow, h.Name, h.Symbol,
						newPrice, changePct,
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
		_ = client.SendMessage(msg)
		slog.Info("sent price alert", "userId", userID, "portfolioId", portfolioID, "count", len(alerts))
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

	driftThreshold := 5.0
	if v := settings["driftThreshold"]; v != "" {
		_, _ = fmt.Sscanf(v, "%f", &driftThreshold)
	}

	var holdings []models.Holding
	if err := n.db.Where("portfolio_id = ?", portfolioID).Find(&holdings).Error; err != nil {
		return
	}

	assets := map[string]float64{
		"stocks":      0,
		"bonds":       0,
		"cash":        0,
		"commodities": 0,
	}
	var total float64
	for i := range holdings {
		h := &holdings[i]
		assets[h.AssetId] += h.Value
		total += h.Value
	}

	if total == 0 {
		return
	}

	targetPcts := map[string]float64{
		"stocks":      25.0,
		"bonds":       25.0,
		"cash":        25.0,
		"commodities": 25.0,
	}
	for id := range targetPcts {
		if v := settings["target"+strings.ToUpper(id[:1])+id[1:]]; v != "" {
			var pct float64
			if _, err := fmt.Sscanf(v, "%f", &pct); err == nil {
				targetPcts[id] = pct
			}
		}
	}

	var targetTotal float64
	for _, v := range targetPcts {
		targetTotal += v
	}
	if targetTotal > 0 && targetTotal != 100 {
		for id := range targetPcts {
			targetPcts[id] = targetPcts[id] / targetTotal * 100
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
		pct := value / total * 100
		diff := pct - targetPcts[id]
		if diff > driftThreshold || diff < -driftThreshold {
			alerts = append(alerts, fmt.Sprintf(
				"<b>%s</b>: %.1f%% (目标 %.0f%%, 偏离 %+.1f%%)",
				assetNames[id], pct, targetPcts[id], diff,
			))
		}
	}

	if len(alerts) > 0 {
		msg := "⚠️ <b>配比偏离提醒</b>\n\n当前资产配置:\n" + strings.Join(alerts, "\n")
		_ = client.SendMessage(msg)
		n.mu.Lock()
		n.lastDriftAlert[cacheKey] = time.Now()
		n.mu.Unlock()
		slog.Info("sent drift alert", "userId", userID, "portfolioId", portfolioID, "count", len(alerts))
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

	assets := map[string]float64{
		"stocks":      0,
		"bonds":       0,
		"cash":        0,
		"commodities": 0,
	}
	var total, totalCost, totalBuyFees float64
	for i := range holdings {
		h := &holdings[i]
		assets[h.AssetId] += h.Value
		total += h.Value
		totalCost += h.Cost
		totalBuyFees += h.BuyFees()
	}
	principal := totalCost + totalBuyFees

	assetNames := map[string]string{
		"stocks":      "股票",
		"bonds":       "债券",
		"cash":        "现金",
		"commodities": "商品",
	}

	lines := []string{
		fmt.Sprintf("📊 <b>投资组合摘要</b> — %s", now.Format("2006-01-02")),
		"",
		fmt.Sprintf("总资产: ¥%.0f", total),
		fmt.Sprintf("累计投入: ¥%.0f", principal),
	}
	if principal > 0 {
		pnl := (total - principal) / principal * 100
		lines = append(lines, fmt.Sprintf("累计收益: %+.1f%%", pnl))
	}
	lines = append(lines, "")

	for _, id := range []string{"stocks", "bonds", "cash", "commodities"} {
		pct := 0.0
		if total > 0 {
			pct = assets[id] / total * 100
		}
		lines = append(lines, fmt.Sprintf("%s  %.1f%%  ¥%.0f", assetNames[id], pct, assets[id]))
	}

	_ = client.SendMessage(strings.Join(lines, "\n"))

	n.mu.Lock()
	n.lastSummaryTime[cacheKey] = now
	n.mu.Unlock()
	slog.Info("sent portfolio summary", "userId", userID, "portfolioId", portfolioID)
}
