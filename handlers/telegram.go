package handlers

import (
	"context"
	"fmt"
	"portfolio-management/marketsource"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"portfolio-management/telegram"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestTelegramMessage(db *gorm.DB, router *marketsource.Router) app.HandlerFunc {
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
			user := middleware.GetUser(c)
			if user == nil {
				c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
				return
			}
			holdings, err := gorm.G[models.Holding](db).Where("user_id = ?", user.UserID).Find(ctx)
			if err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": "查询持仓失败: " + err.Error()})
				return
			}

			assets := map[string]decimal.Decimal{"stocks": decimal.Zero, "bonds": decimal.Zero, "cash": decimal.Zero, "commodities": decimal.Zero}
			total := decimal.Zero
			for i := range holdings {
				h := &holdings[i]
				assets[h.AssetId] = assets[h.AssetId].Add(h.Value)
				total = total.Add(h.Value)
			}
			principal, err := CalcPrincipalByUser(db, user.UserID, "CNY", router)
			if err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": "计算累计投入失败: " + err.Error()})
				return
			}

			assetNames := map[string]string{
				"stocks": "股票", "bonds": "债券", "cash": "现金", "commodities": "商品",
			}

			now := time.Now()
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
