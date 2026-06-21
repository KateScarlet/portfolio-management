package handlers

import (
	"context"
	"fmt"
	"permanent-portfolio/models"
	"permanent-portfolio/telegram"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"gorm.io/gorm"
)

func TestTelegramMessage(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var body struct {
			BotToken string `json:"botToken"`
			ChatID   string `json:"chatID"`
			Type     string `json:"type"` // connection, price, drift, summary
		}
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if body.BotToken == "" || body.ChatID == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "botToken and chatID are required"})
			return
		}

		client, err := telegram.NewClient(body.BotToken, body.ChatID)
		if err != nil {
			c.JSON(consts.StatusOK, map[string]any{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		switch body.Type {
		case "connection", "":
			botName := client.BotName()
			msg := fmt.Sprintf("✅ <b>连接测试成功</b>\n\nBot: %s\n<i>— 这是一条测试消息</i>", botName)
			if err := client.SendMessage(msg); err != nil {
				c.JSON(consts.StatusOK, map[string]any{"success": false, "error": err.Error()})
				return
			}

		case "price":
			msg := `⚠️ <b>价格波动提醒</b>

📈 <b>沪深300ETF</b> (510300)
当前价: ¥4.56 | 涨跌: +6.2%

📉 <b>国债ETF</b> (511010)
当前价: ¥102.30 | 涨跌: -5.5%

<i>— 这是一条测试消息</i>`
			if err := client.SendMessage(msg); err != nil {
				c.JSON(consts.StatusOK, map[string]any{"success": false, "error": err.Error()})
				return
			}

		case "drift":
			msg := `⚠️ <b>配比偏离提醒</b>

当前资产配置 vs 目标 25%:
<b>股票</b>: 35.2% (偏离 +10.2%)
<b>债券</b>: 14.8% (偏离 -10.2%)

<i>— 这是一条测试消息</i>`
			if err := client.SendMessage(msg); err != nil {
				c.JSON(consts.StatusOK, map[string]any{"success": false, "error": err.Error()})
				return
			}

		case "summary":
			var holdings []models.Holding
			_ = db.Find(&holdings).Error

			assets := map[string]float64{"stocks": 0, "bonds": 0, "cash": 0, "gold": 0}
			var total, totalCost float64
			for i := range holdings {
				h := &holdings[i]
				assets[h.AssetId] += h.Value
				total += h.Value
				totalCost += h.Cost
			}

			assetNames := map[string]string{
				"stocks": "股票", "bonds": "债券", "cash": "现金", "gold": "商品",
			}

			now := time.Now()
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
			lines = append(lines, "", "<i>— 这是一条测试消息</i>")

			if err := client.SendMessage(strings.Join(lines, "\n")); err != nil {
				c.JSON(consts.StatusOK, map[string]any{"success": false, "error": err.Error()})
				return
			}

		default:
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid test type: " + body.Type})
			return
		}

		c.JSON(consts.StatusOK, map[string]any{"success": true})
	}
}
