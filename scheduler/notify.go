package scheduler

import (
	"fmt"
	"log/slog"
	"maps"
	"permanent-portfolio/models"
	"permanent-portfolio/telegram"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Notifier struct {
	db              *gorm.DB
	prevPrices      map[string]float64
	lastDriftAlert  time.Time
	lastSummaryTime time.Time
	telegramClient  *telegram.Client
	telegramToken   string
	telegramChatID  string
}

func NewNotifier(db *gorm.DB) *Notifier {
	return &Notifier{
		db:         db,
		prevPrices: make(map[string]float64),
	}
}

func (n *Notifier) LoadTelegramConfig() (*telegram.Client, error) {
	var token, chatID string
	var enabled string

	_ = n.db.Model(&models.Setting{}).Where("key = ?", "telegramBotToken").Select("value").Row().Scan(&token)
	_ = n.db.Model(&models.Setting{}).Where("key = ?", "telegramChatID").Select("value").Row().Scan(&chatID)
	_ = n.db.Model(&models.Setting{}).Where("key = ?", "telegramEnabled").Select("value").Row().Scan(&enabled)

	if enabled != "true" || token == "" || chatID == "" {
		n.telegramClient = nil
		return nil, nil
	}

	if n.telegramClient != nil && n.telegramToken == token && n.telegramChatID == chatID {
		return n.telegramClient, nil
	}

	client, err := telegram.NewClient(token, chatID)
	if err != nil {
		return nil, err
	}

	n.telegramClient = client
	n.telegramToken = token
	n.telegramChatID = chatID
	return client, nil
}

func (n *Notifier) NotifyAfterSync(holdings []models.Holding, syncedSymbols map[string]float64) {
	client, err := n.LoadTelegramConfig()
	if err != nil {
		slog.Error("failed to load telegram config for notification", "error", err)
		return
	}
	if client == nil {
		return
	}

	n.checkPriceAlert(client, holdings, syncedSymbols)
	n.checkDriftAlert(client)
	n.checkSummary(client, holdings)
}

func (n *Notifier) checkPriceAlert(client *telegram.Client, holdings []models.Holding, syncedPrices map[string]float64) {
	var enabled string
	_ = n.db.Model(&models.Setting{}).Where("key = ?", "telegramPriceAlert").Select("value").Row().Scan(&enabled)
	if enabled != "true" {
		return
	}

	var thresholdStr string
	_ = n.db.Model(&models.Setting{}).Where("key = ?", "telegramPriceThreshold").Select("value").Row().Scan(&thresholdStr)
	threshold := 5.0
	if thresholdStr != "" {
		_, _ = fmt.Sscanf(thresholdStr, "%f", &threshold)
	}

	var alerts []string
	for symbol, newPrice := range syncedPrices {
		oldPrice, ok := n.prevPrices[symbol]
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

	maps.Copy(n.prevPrices, syncedPrices)

	if len(alerts) > 0 {
		msg := "⚠️ <b>价格波动提醒</b>\n\n" + strings.Join(alerts, "\n\n")
		_ = client.SendMessage(msg)
		slog.Info("sent price alert", "count", len(alerts))
	}
}

func (n *Notifier) checkDriftAlert(client *telegram.Client) {
	var enabled string
	_ = n.db.Model(&models.Setting{}).Where("key = ?", "telegramDriftAlert").Select("value").Row().Scan(&enabled)
	if enabled != "true" {
		return
	}

	if time.Since(n.lastDriftAlert) < 24*time.Hour {
		return
	}

	var driftThresholdStr string
	_ = n.db.Model(&models.Setting{}).Where("key = ?", "driftThreshold").Select("value").Row().Scan(&driftThresholdStr)
	driftThreshold := 5.0
	if driftThresholdStr != "" {
		_, _ = fmt.Sscanf(driftThresholdStr, "%f", &driftThreshold)
	}

	var holdings []models.Holding
	if err := n.db.Find(&holdings).Error; err != nil {
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
		n.lastDriftAlert = time.Now()
		slog.Info("sent drift alert", "count", len(alerts))
	}
}

func (n *Notifier) checkSummary(client *telegram.Client, holdings []models.Holding) {
	var enabled string
	_ = n.db.Model(&models.Setting{}).Where("key = ?", "telegramSummary").Select("value").Row().Scan(&enabled)
	if enabled != "true" {
		return
	}

	var interval string
	_ = n.db.Model(&models.Setting{}).Where("key = ?", "telegramSummaryInterval").Select("value").Row().Scan(&interval)

	shouldSend := false
	now := time.Now()

	switch interval {
	case "daily":
		if n.lastSummaryTime.IsZero() || now.Sub(n.lastSummaryTime) >= 24*time.Hour {
			shouldSend = true
		}
	case "weekly":
		if n.lastSummaryTime.IsZero() || now.Sub(n.lastSummaryTime) >= 7*24*time.Hour {
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

	n.lastSummaryTime = now
	slog.Info("sent portfolio summary")
}
