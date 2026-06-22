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

type Notifier struct {
	db              *gorm.DB
	mu              sync.RWMutex
	prevPrices      map[string]map[string]float64
	lastDriftAlert  map[string]time.Time
	lastSummaryTime map[string]time.Time
}

func NewNotifier(db *gorm.DB) *Notifier {
	return &Notifier{
		db:              db,
		prevPrices:      make(map[string]map[string]float64),
		lastDriftAlert:  make(map[string]time.Time),
		lastSummaryTime: make(map[string]time.Time),
	}
}

func (n *Notifier) LoadTelegramConfig(userID string) (*telegram.Client, error) {
	var token, chatID string
	var enabled string

	_ = n.db.Model(&models.Setting{}).Where("key = ? AND user_id = ?", "telegramBotToken", userID).Select("value").Row().Scan(&token)
	_ = n.db.Model(&models.Setting{}).Where("key = ? AND user_id = ?", "telegramChatID", userID).Select("value").Row().Scan(&chatID)
	_ = n.db.Model(&models.Setting{}).Where("key = ? AND user_id = ?", "telegramEnabled", userID).Select("value").Row().Scan(&enabled)

	if enabled != "true" || token == "" || chatID == "" {
		return nil, nil
	}

	client, err := telegram.NewClient(token, chatID)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (n *Notifier) NotifyAfterSync(userID string, holdings []models.Holding, syncedSymbols map[string]float64) {
	client, err := n.LoadTelegramConfig(userID)
	if err != nil {
		slog.Error("failed to load telegram config for notification", "userId", userID, "error", err)
		return
	}
	if client == nil {
		return
	}

	n.checkPriceAlert(userID, client, holdings, syncedSymbols)
	n.checkDriftAlert(userID, client)
	n.checkSummary(userID, client, holdings)
}

func (n *Notifier) checkPriceAlert(userID string, client *telegram.Client, holdings []models.Holding, syncedPrices map[string]float64) {
	var enabled string
	_ = n.db.Model(&models.Setting{}).Where("key = ? AND user_id = ?", "telegramPriceAlert", userID).Select("value").Row().Scan(&enabled)
	if enabled != "true" {
		return
	}

	var thresholdStr string
	_ = n.db.Model(&models.Setting{}).Where("key = ? AND user_id = ?", "telegramPriceThreshold", userID).Select("value").Row().Scan(&thresholdStr)
	threshold := 5.0
	if thresholdStr != "" {
		_, _ = fmt.Sscanf(thresholdStr, "%f", &threshold)
	}

	n.mu.Lock()
	if n.prevPrices[userID] == nil {
		n.prevPrices[userID] = make(map[string]float64)
	}
	oldPrices := n.prevPrices[userID]

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
		slog.Info("sent price alert", "userId", userID, "count", len(alerts))
	}
}

func (n *Notifier) checkDriftAlert(userID string, client *telegram.Client) {
	var enabled string
	_ = n.db.Model(&models.Setting{}).Where("key = ? AND user_id = ?", "telegramDriftAlert", userID).Select("value").Row().Scan(&enabled)
	if enabled != "true" {
		return
	}

	n.mu.RLock()
	lastAlert, exists := n.lastDriftAlert[userID]
	n.mu.RUnlock()

	if exists && time.Since(lastAlert) < 24*time.Hour {
		return
	}

	var driftThresholdStr string
	_ = n.db.Model(&models.Setting{}).Where("key = ? AND user_id = ?", "driftThreshold", userID).Select("value").Row().Scan(&driftThresholdStr)
	driftThreshold := 5.0
	if driftThresholdStr != "" {
		_, _ = fmt.Sscanf(driftThresholdStr, "%f", &driftThreshold)
	}

	var holdings []models.Holding
	if err := n.db.Where("user_id = ?", userID).Find(&holdings).Error; err != nil {
		return
	}

	assets := map[string]float64{
		"stocks": 0,
		"bonds":  0,
		"cash":   0,
		"gold":   0,
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

	targetPct := 25.0
	var alerts []string
	assetNames := map[string]string{
		"stocks": "股票",
		"bonds":  "债券",
		"cash":   "现金",
		"gold":   "商品",
	}

	for id, value := range assets {
		pct := value / total * 100
		diff := pct - targetPct
		if diff > driftThreshold || diff < -driftThreshold {
			alerts = append(alerts, fmt.Sprintf(
				"<b>%s</b>: %.1f%% (偏离 %+.1f%%)",
				assetNames[id], pct, diff,
			))
		}
	}

	if len(alerts) > 0 {
		msg := "⚠️ <b>配比偏离提醒</b>\n\n当前资产配置 vs 目标 25%:\n" + strings.Join(alerts, "\n")
		_ = client.SendMessage(msg)
		n.mu.Lock()
		n.lastDriftAlert[userID] = time.Now()
		n.mu.Unlock()
		slog.Info("sent drift alert", "userId", userID, "count", len(alerts))
	}
}

func (n *Notifier) checkSummary(userID string, client *telegram.Client, holdings []models.Holding) {
	var enabled string
	_ = n.db.Model(&models.Setting{}).Where("key = ? AND user_id = ?", "telegramSummary", userID).Select("value").Row().Scan(&enabled)
	if enabled != "true" {
		return
	}

	var interval string
	_ = n.db.Model(&models.Setting{}).Where("key = ? AND user_id = ?", "telegramSummaryInterval", userID).Select("value").Row().Scan(&interval)

	shouldSend := false
	now := time.Now()

	n.mu.RLock()
	lastTime, exists := n.lastSummaryTime[userID]
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
		"stocks": 0,
		"bonds":  0,
		"cash":   0,
		"gold":   0,
	}
	var total, totalCost float64
	for i := range holdings {
		h := &holdings[i]
		assets[h.AssetId] += h.Value
		total += h.Value
		totalCost += h.Cost
	}

	assetNames := map[string]string{
		"stocks": "股票",
		"bonds":  "债券",
		"cash":   "现金",
		"gold":   "商品",
	}

	lines := []string{
		fmt.Sprintf("📊 <b>投资组合摘要</b> — %s", now.Format("2006-01-02")),
		"",
		fmt.Sprintf("总资产: ¥%.0f", total),
		fmt.Sprintf("总成本: ¥%.0f", totalCost),
	}
	if totalCost > 0 {
		pnl := (total - totalCost) / totalCost * 100
		lines = append(lines, fmt.Sprintf("累计收益: %+.1f%%", pnl))
	}
	lines = append(lines, "")

	for _, id := range []string{"stocks", "bonds", "cash", "gold"} {
		pct := 0.0
		if total > 0 {
			pct = assets[id] / total * 100
		}
		lines = append(lines, fmt.Sprintf("%s  %.1f%%  ¥%.0f", assetNames[id], pct, assets[id]))
	}

	_ = client.SendMessage(strings.Join(lines, "\n"))

	n.mu.Lock()
	n.lastSummaryTime[userID] = now
	n.mu.Unlock()
	slog.Info("sent portfolio summary", "userId", userID)
}
