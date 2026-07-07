package scheduler

import (
	"fmt"
	"log/slog"
	"maps"
	"portfolio-management/bark"
	"portfolio-management/marketsource"
	"portfolio-management/models"
	"portfolio-management/telegram"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type cachedTelegram struct {
	client *telegram.Client
	token  string
	chatID string
}

type cachedBark struct {
	client    *bark.Client
	deviceKey string
	serverURL string
}

type Notifier struct {
	db              *gorm.DB
	router          *marketsource.Router
	mu              sync.RWMutex
	prevPrices      map[uuid.UUID]map[string]decimal.Decimal
	lastDriftAlert  map[uuid.UUID]time.Time
	lastSummaryTime map[uuid.UUID]time.Time
	telegramClients map[uuid.UUID]*cachedTelegram
	barkClients     map[uuid.UUID]*cachedBark
}

func NewNotifier(db *gorm.DB, router *marketsource.Router) *Notifier {
	return &Notifier{
		db:              db,
		router:          router,
		prevPrices:      make(map[uuid.UUID]map[string]decimal.Decimal),
		lastDriftAlert:  make(map[uuid.UUID]time.Time),
		lastSummaryTime: make(map[uuid.UUID]time.Time),
		telegramClients: make(map[uuid.UUID]*cachedTelegram),
		barkClients:     make(map[uuid.UUID]*cachedBark),
	}
}

func (n *Notifier) loadPortfolioSettings(portfolioID uuid.UUID) map[string]string {
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

func (n *Notifier) loadUserTelegramConfig(userID uuid.UUID) (token, chatID, enabled string) {
	var settings []models.Setting
	if err := n.db.Where(`user_id = ? AND "key" IN ('telegramBotToken', 'telegramChatID', 'telegramEnabled')`, userID).Find(&settings).Error; err != nil {
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

func (n *Notifier) LoadTelegramConfig(userID uuid.UUID) (*telegram.Client, error) {
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

func (n *Notifier) loadUserBarkConfig(userID uuid.UUID) (deviceKey, serverURL, enabled string) {
	var settings []models.Setting
	if err := n.db.Where(`user_id = ? AND "key" IN ('barkDeviceKey', 'barkServerURL', 'barkEnabled')`, userID).Find(&settings).Error; err != nil {
		slog.Error("failed to load user bark config", "userId", userID, "error", err)
	}
	for _, s := range settings {
		switch s.Key {
		case "barkDeviceKey":
			deviceKey = s.Value
		case "barkServerURL":
			serverURL = s.Value
		case "barkEnabled":
			enabled = s.Value
		}
	}
	return
}

func (n *Notifier) LoadBarkConfig(userID uuid.UUID) (*bark.Client, error) {
	deviceKey, serverURL, enabled := n.loadUserBarkConfig(userID)

	if enabled != "true" || deviceKey == "" {
		n.mu.Lock()
		delete(n.barkClients, userID)
		n.mu.Unlock()
		return nil, nil
	}

	n.mu.RLock()
	cached, ok := n.barkClients[userID]
	n.mu.RUnlock()
	if ok && cached.deviceKey == deviceKey && cached.serverURL == serverURL {
		return cached.client, nil
	}

	client, err := bark.NewClient(deviceKey, serverURL)
	if err != nil {
		return nil, err
	}

	n.mu.Lock()
	n.barkClients[userID] = &cachedBark{client: client, deviceKey: deviceKey, serverURL: serverURL}
	n.mu.Unlock()

	return client, nil
}

func (n *Notifier) NotifyAfterSync(userID, portfolioID uuid.UUID, holdings []models.Holding, syncedSymbols map[string]decimal.Decimal) {
	tgClient, _ := n.LoadTelegramConfig(userID)
	barkClient, _ := n.LoadBarkConfig(userID)

	if tgClient == nil && barkClient == nil {
		return
	}

	n.checkPriceAlert(userID, portfolioID, tgClient, barkClient, holdings, syncedSymbols)
	n.checkDriftAlert(userID, portfolioID, tgClient, barkClient)
	n.checkSummary(userID, portfolioID, tgClient, barkClient, holdings)
}

func (n *Notifier) checkPriceAlert(userID, portfolioID uuid.UUID, tgClient *telegram.Client, barkClient *bark.Client, holdings []models.Holding, syncedPrices map[string]decimal.Decimal) {
	settings := n.loadPortfolioSettings(portfolioID)

	tgEnabled := tgClient != nil && settings["telegramPriceAlert"] == "true"
	barkEnabled := barkClient != nil && settings["barkPriceAlert"] == "true"

	if !tgEnabled && !barkEnabled {
		return
	}

	threshold := decimal.NewFromInt(5)
	if v := settings["telegramPriceThreshold"]; v != "" {
		if t, err := decimal.NewFromString(v); err == nil {
			threshold = t
		}
	}
	if v := settings["barkPriceThreshold"]; v != "" {
		if t, err := decimal.NewFromString(v); err == nil {
			threshold = t
		}
	}

	n.mu.Lock()
	if n.prevPrices[userID] == nil {
		n.prevPrices[userID] = make(map[string]decimal.Decimal)
	}
	oldPrices := n.prevPrices[userID]

	var tgAlerts []string
	var barkAlerts []string
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
					if tgEnabled {
						tgAlerts = append(tgAlerts, fmt.Sprintf(
							"%s <b>%s</b> (%s)\n当前价: ¥%s | 涨跌: %s%%",
							arrow, h.Name, h.Symbol,
							newPrice.StringFixed(2), changePct.StringFixed(1),
						))
					}
					if barkEnabled {
						barkAlerts = append(barkAlerts, fmt.Sprintf(
							"%s %s (%s)\n当前价: ¥%s | 涨跌: %s%%",
							arrow, h.Name, h.Symbol,
							newPrice.StringFixed(2), changePct.StringFixed(1),
						))
					}
					break
				}
			}
		}
	}

	maps.Copy(oldPrices, syncedPrices)
	n.mu.Unlock()

	if len(tgAlerts) > 0 && tgClient != nil {
		msg := "⚠️ <b>价格波动提醒</b>\n\n" + strings.Join(tgAlerts, "\n\n")
		if err := tgClient.SendMessage(msg); err != nil {
			slog.Error("failed to send price alert via telegram", "userId", userID, "portfolioId", portfolioID, "error", err)
		} else {
			slog.Info("sent price alert via telegram", "userId", userID, "portfolioId", portfolioID, "count", len(tgAlerts))
		}
	}

	if len(barkAlerts) > 0 && barkClient != nil {
		msg := strings.Join(barkAlerts, "\n\n")
		if err := barkClient.SendNotification("价格波动提醒", msg, "价格告警"); err != nil {
			slog.Error("failed to send price alert via bark", "userId", userID, "portfolioId", portfolioID, "error", err)
		} else {
			slog.Info("sent price alert via bark", "userId", userID, "portfolioId", portfolioID, "count", len(barkAlerts))
		}
	}
}

func (n *Notifier) checkDriftAlert(userID, portfolioID uuid.UUID, tgClient *telegram.Client, barkClient *bark.Client) {
	settings := n.loadPortfolioSettings(portfolioID)

	tgEnabled := tgClient != nil && settings["telegramDriftAlert"] == "true"
	barkEnabled := barkClient != nil && settings["barkDriftAlert"] == "true"

	if !tgEnabled && !barkEnabled {
		return
	}

	n.mu.RLock()
	lastAlert, exists := n.lastDriftAlert[userID]
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

	var tgAlerts []string
	var barkAlerts []string
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
			if tgEnabled {
				tgAlerts = append(tgAlerts, fmt.Sprintf(
					"<b>%s</b>: %s%% (目标 %s%%, 偏离 %s%%)",
					assetNames[id], pct.StringFixed(1), targetPcts[id].StringFixed(0), diff.StringFixed(1),
				))
			}
			if barkEnabled {
				barkAlerts = append(barkAlerts, fmt.Sprintf(
					"%s: %s%% (目标 %s%%, 偏离 %s%%)",
					assetNames[id], pct.StringFixed(1), targetPcts[id].StringFixed(0), diff.StringFixed(1),
				))
			}
		}
	}

	if len(tgAlerts) > 0 && tgClient != nil {
		msg := "⚠️ <b>配比偏离提醒</b>\n\n当前资产配置:\n" + strings.Join(tgAlerts, "\n")
		if err := tgClient.SendMessage(msg); err != nil {
			slog.Error("failed to send drift alert via telegram", "userId", userID, "portfolioId", portfolioID, "error", err)
		} else {
			n.mu.Lock()
			n.lastDriftAlert[userID] = time.Now()
			n.mu.Unlock()
			slog.Info("sent drift alert via telegram", "userId", userID, "portfolioId", portfolioID, "count", len(tgAlerts))
		}
	}

	if len(barkAlerts) > 0 && barkClient != nil {
		msg := "当前资产配置:\n" + strings.Join(barkAlerts, "\n")
		if err := barkClient.SendNotification("配比偏离提醒", msg, "配比偏离"); err != nil {
			slog.Error("failed to send drift alert via bark", "userId", userID, "portfolioId", portfolioID, "error", err)
		} else {
			n.mu.Lock()
			n.lastDriftAlert[userID] = time.Now()
			n.mu.Unlock()
			slog.Info("sent drift alert via bark", "userId", userID, "portfolioId", portfolioID, "count", len(barkAlerts))
		}
	}
}

func (n *Notifier) checkSummary(userID, portfolioID uuid.UUID, tgClient *telegram.Client, barkClient *bark.Client, holdings []models.Holding) {
	settings := n.loadPortfolioSettings(portfolioID)

	tgEnabled := tgClient != nil && settings["telegramSummary"] == "true"
	barkEnabled := barkClient != nil && settings["barkSummary"] == "true"

	if !tgEnabled && !barkEnabled {
		return
	}

	tgInterval := settings["telegramSummaryInterval"]
	barkInterval := settings["barkSummaryInterval"]

	tgShouldSend := false
	barkShouldSend := false
	now := time.Now()

	n.mu.RLock()
	lastTime, exists := n.lastSummaryTime[userID]
	n.mu.RUnlock()

	if tgEnabled {
		switch tgInterval {
		case "daily":
			if !exists || now.Sub(lastTime) >= 24*time.Hour {
				tgShouldSend = true
			}
		case "weekly":
			if !exists || now.Sub(lastTime) >= 7*24*time.Hour {
				tgShouldSend = true
			}
		}
	}

	if barkEnabled {
		switch barkInterval {
		case "daily":
			if !exists || now.Sub(lastTime) >= 24*time.Hour {
				barkShouldSend = true
			}
		case "weekly":
			if !exists || now.Sub(lastTime) >= 7*24*time.Hour {
				barkShouldSend = true
			}
		}
	}

	if !tgShouldSend && !barkShouldSend {
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

	tgLines := []string{
		fmt.Sprintf("📊 <b>投资组合摘要</b> — %s", now.Format("2006-01-02")),
		"",
		fmt.Sprintf("总资产: ¥%s", total.StringFixed(0)),
		fmt.Sprintf("累计投入: ¥%s", principal.StringFixed(0)),
	}
	barkLines := []string{
		fmt.Sprintf("📊 投资组合摘要 — %s", now.Format("2006-01-02")),
		"",
		fmt.Sprintf("总资产: ¥%s", total.StringFixed(0)),
		fmt.Sprintf("累计投入: ¥%s", principal.StringFixed(0)),
	}
	if principal.IsPositive() {
		pnl := total.Sub(principal).Div(principal).Mul(decimal.NewFromInt(100))
		tgLines = append(tgLines, fmt.Sprintf("累计收益: %s%%", pnl.StringFixed(1)))
		barkLines = append(barkLines, fmt.Sprintf("累计收益: %s%%", pnl.StringFixed(1)))
	}
	tgLines = append(tgLines, "")
	barkLines = append(barkLines, "")

	for _, id := range []string{"stocks", "bonds", "cash", "commodities"} {
		pct := decimal.Zero
		if total.IsPositive() {
			pct = assets[id].Div(total).Mul(decimal.NewFromInt(100))
		}
		tgLines = append(tgLines, fmt.Sprintf("%s  %s%%  ¥%s", assetNames[id], pct.StringFixed(1), assets[id].StringFixed(0)))
		barkLines = append(barkLines, fmt.Sprintf("%s  %s%%  ¥%s", assetNames[id], pct.StringFixed(1), assets[id].StringFixed(0)))
	}

	if tgShouldSend && tgClient != nil {
		if err := tgClient.SendMessage(strings.Join(tgLines, "\n")); err != nil {
			slog.Error("failed to send portfolio summary via telegram", "userId", userID, "portfolioId", portfolioID, "error", err)
		} else {
			n.mu.Lock()
			n.lastSummaryTime[userID] = now
			n.mu.Unlock()
			slog.Info("sent portfolio summary via telegram", "userId", userID, "portfolioId", portfolioID)
		}
	}

	if barkShouldSend && barkClient != nil {
		if err := barkClient.SendNotification("投资组合摘要", strings.Join(barkLines, "\n"), "组合摘要"); err != nil {
			slog.Error("failed to send portfolio summary via bark", "userId", userID, "portfolioId", portfolioID, "error", err)
		} else {
			n.mu.Lock()
			n.lastSummaryTime[userID] = now
			n.mu.Unlock()
			slog.Info("sent portfolio summary via bark", "userId", userID, "portfolioId", portfolioID)
		}
	}
}
