package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"portfolio-management/bark"
	"portfolio-management/marketsource"
	"portfolio-management/middleware"
	"portfolio-management/models"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestBarkNotification(db *gorm.DB, router *marketsource.Router) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var body struct {
			DeviceKey string `json:"deviceKey"`
			ServerURL string `json:"serverURL"`
			Type      string `json:"type"` // connection, price, drift, summary
		}
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if body.DeviceKey == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "deviceKey 不能为空"})
			return
		}

		client, err := bark.NewClient(body.DeviceKey, body.ServerURL)
		if err != nil {
			c.JSON(consts.StatusOK, map[string]any{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		switch body.Type {
		case "connection", "":
			if err := client.SendNotification("连接测试", "Bark 通知已连接成功！", "portfolio"); err != nil {
				c.JSON(consts.StatusOK, map[string]any{"success": false, "error": err.Error()})
				return
			}

		case "price":
			msg := `📈 沪深300ETF (510300)
当前价: ¥4.56 | 涨跌: +6.2%

📉 国债ETF (511010)
当前价: ¥102.30 | 涨跌: -5.5%

— 这是一条测试消息`
			if err := client.SendNotification("价格波动提醒", msg, "价格告警"); err != nil {
				c.JSON(consts.StatusOK, map[string]any{"success": false, "error": err.Error()})
				return
			}

		case "drift":
			msg := `当前资产配置 vs 目标 25%:
股票: 35.2% (偏离 +10.2%)
债券: 14.8% (偏离 -10.2%)

— 这是一条测试消息`
			if err := client.SendNotification("配比偏离提醒", msg, "配比偏离"); err != nil {
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
				fmt.Sprintf("📊 投资组合摘要 — %s", now.Format("2006-01-02")),
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
			lines = append(lines, "", "— 这是一条测试消息")

			if err := client.SendNotification("投资组合摘要", strings.Join(lines, "\n"), "组合摘要"); err != nil {
				c.JSON(consts.StatusOK, map[string]any{"success": false, "error": err.Error()})
				return
			}

		default:
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的测试类型: " + body.Type})
			return
		}

		c.JSON(consts.StatusOK, map[string]any{"success": true})
	}
}
