package bark

import (
	"fmt"
	"log/slog"
	"portfolio-management/internal/notifications"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type cachedClient struct {
	client    *Client
	deviceKey string
	serverURL string
}

// Channel implements notifications.Channel for Bark.
type Channel struct {
	db      *gorm.DB
	mu      sync.RWMutex
	clients map[string]*cachedClient
}

// NewChannel creates a new Bark notification channel.
func NewChannel(db *gorm.DB) *Channel {
	return &Channel{
		db:      db,
		clients: make(map[string]*cachedClient),
	}
}

func (c *Channel) Name() string { return "bark" }

func (c *Channel) IsEnabled(userID, portfolioID uuid.UUID, db *gorm.DB) bool {
	return notifications.IsEnabled(db, userID, portfolioID, "barkEnabled")
}

func (c *Channel) ShouldSendSummary(userID, portfolioID uuid.UUID, lastTime, now time.Time, db *gorm.DB) bool {
	return notifications.ShouldSendSummary(db, userID, portfolioID, "barkSummaryInterval", lastTime, now)
}

func (c *Channel) getClient(userID, portfolioID uuid.UUID) (*Client, error) {
	deviceKey := notifications.LoadSetting(c.db, userID, portfolioID, "barkDeviceKey")
	serverURL := notifications.LoadSetting(c.db, userID, portfolioID, "barkServerURL")

	key := notifications.StateKey(userID, portfolioID)

	if deviceKey == "" {
		c.mu.Lock()
		delete(c.clients, key)
		c.mu.Unlock()
		return nil, nil
	}

	c.mu.RLock()
	cached, ok := c.clients[key]
	c.mu.RUnlock()
	if ok && cached.deviceKey == deviceKey && cached.serverURL == serverURL {
		return cached.client, nil
	}

	client, err := NewClient(deviceKey, serverURL)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.clients[key] = &cachedClient{client: client, deviceKey: deviceKey, serverURL: serverURL}
	c.mu.Unlock()

	return client, nil
}

func (c *Channel) SendPriceAlert(userID, portfolioID uuid.UUID, portfolioName string, alerts []notifications.PriceAlert, db *gorm.DB) error {
	if len(alerts) == 0 {
		return nil
	}

	client, err := c.getClient(userID, portfolioID)
	if err != nil {
		return err
	}
	if client == nil {
		return nil
	}

	lines := make([]string, len(alerts))
	for i, a := range alerts {
		lines[i] = fmt.Sprintf(
			"%s %s (%s)\n当前价: %s | 涨跌: %s%%",
			a.Arrow, a.Name, a.Symbol, a.Price, a.ChangePct,
		)
	}

	title := "价格波动提醒"
	if portfolioName != "" {
		title += " — " + portfolioName
	}
	msg := strings.Join(lines, "\n\n")

	if err := client.SendNotification(title, msg, "价格告警"); err != nil {
		slog.Error("failed to send price alert via bark", "userId", userID, "portfolioId", portfolioID, "error", err)
		return err
	}
	slog.Info("sent price alert via bark", "userId", userID, "portfolioId", portfolioID, "count", len(alerts))
	return nil
}

func (c *Channel) SendDriftAlert(userID, portfolioID uuid.UUID, portfolioName string, alerts []notifications.DriftAlert, db *gorm.DB) error {
	if len(alerts) == 0 {
		return nil
	}

	client, err := c.getClient(userID, portfolioID)
	if err != nil {
		return err
	}
	if client == nil {
		return nil
	}

	lines := make([]string, len(alerts))
	for i, a := range alerts {
		lines[i] = fmt.Sprintf(
			"%s: %s%% (目标 %s%%, 偏离 %s%%)",
			a.AssetName, a.Pct, a.Target, a.Diff,
		)
	}

	title := "配比偏离提醒"
	if portfolioName != "" {
		title += " — " + portfolioName
	}
	msg := "当前资产配置:\n" + strings.Join(lines, "\n")

	if err := client.SendNotification(title, msg, "配比偏离"); err != nil {
		slog.Error("failed to send drift alert via bark", "userId", userID, "portfolioId", portfolioID, "error", err)
		return err
	}
	slog.Info("sent drift alert via bark", "userId", userID, "portfolioId", portfolioID, "count", len(alerts))
	return nil
}

func (c *Channel) SendSummary(userID, portfolioID uuid.UUID, portfolioName string, data notifications.SummaryData, db *gorm.DB) error {
	client, err := c.getClient(userID, portfolioID)
	if err != nil {
		return err
	}
	if client == nil {
		return nil
	}

	lines := []string{
		fmt.Sprintf("📊 投资组合摘要 — %s — %s", portfolioName, data.Date),
		"",
		fmt.Sprintf("总资产: ¥%s", data.Total),
		fmt.Sprintf("累计投入: ¥%s", data.Principal),
	}
	if data.PnL != "" {
		lines = append(lines, fmt.Sprintf("累计收益: %s%%", data.PnL))
	}
	lines = append(lines, "")

	for _, a := range data.Assets {
		lines = append(lines, fmt.Sprintf("%s  %s%%  ¥%s", a.Name, a.Pct, a.Value))
		for _, h := range a.Holdings {
			line := fmt.Sprintf("  · %s  %s%%  ¥%s", h.Name, h.Pct, h.Value)
			if h.PnL != "" {
				line += fmt.Sprintf("  %s%%", h.PnL)
			}
			lines = append(lines, line)
		}
	}

	title := "投资组合摘要"
	if portfolioName != "" {
		title += " — " + portfolioName
	}

	if err := client.SendNotification(title, strings.Join(lines, "\n"), "组合摘要"); err != nil {
		slog.Error("failed to send portfolio summary via bark", "userId", userID, "portfolioId", portfolioID, "error", err)
		return err
	}
	slog.Info("sent portfolio summary via bark", "userId", userID, "portfolioId", portfolioID)
	return nil
}
