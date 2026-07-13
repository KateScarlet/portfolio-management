package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"portfolio-management/internal/marketsource"
	"portfolio-management/internal/notifications/bark"
	"portfolio-management/middleware"
	"portfolio-management/models"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestBarkNotification(db *gorm.DB, router *marketsource.Router) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var body struct {
			DeviceKey   string `json:"deviceKey"`
			ServerURL   string `json:"serverURL"`
			Type        string `json:"type"` // connection, price, drift, summary
			PortfolioID string `json:"portfolioId"`
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
			user := middleware.GetUser(c)
			if user == nil {
				c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
				return
			}

			portfolioID, err := uuid.Parse(body.PortfolioID)
			if err != nil {
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的组合ID"})
				return
			}
			owned, err := userOwnsPortfolio(db, user.UserID, portfolioID)
			if err != nil || !owned {
				c.JSON(consts.StatusForbidden, map[string]string{"error": "无权访问该组合"})
				return
			}

			var holdings []models.Holding
			if err := db.Where("user_id = ? AND portfolio_id = ?", user.UserID, portfolioID).Limit(5).Find(&holdings).Error; err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": "查询持仓失败: " + err.Error()})
				return
			}

			if len(holdings) == 0 {
				c.JSON(consts.StatusOK, map[string]any{"success": false, "error": "无持仓数据"})
				return
			}

			lines := []string{}
			for i := range holdings {
				h := &holdings[i]
				lines = append(lines, fmt.Sprintf("%s (%s)\n当前价: %s %s",
					h.Name, h.Symbol, h.Currency, h.Price.StringFixed(2)))
			}
			lines = append(lines, "", "— 这是一条测试消息")

			if err := client.SendNotification("价格波动提醒", strings.Join(lines, "\n\n"), "价格告警"); err != nil {
				c.JSON(consts.StatusOK, map[string]any{"success": false, "error": err.Error()})
				return
			}

		case "drift":
			user := middleware.GetUser(c)
			if user == nil {
				c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
				return
			}

			portfolioID, err := uuid.Parse(body.PortfolioID)
			if err != nil {
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的组合ID"})
				return
			}

			var portfolio models.Portfolio
			if err := db.Where("id = ? AND user_id = ?", portfolioID, user.UserID).First(&portfolio).Error; err != nil {
				c.JSON(consts.StatusNotFound, map[string]string{"error": "组合不存在"})
				return
			}

			var holdings []models.Holding
			if err := db.Where("portfolio_id = ?", portfolio.ID).Find(&holdings).Error; err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": "查询持仓失败: " + err.Error()})
				return
			}

			for i := range holdings {
				h := &holdings[i]
				if h.Currency != "" && h.Currency != "CNY" {
					pair := h.Currency + "CNY"
					rate, err := router.ExchangeRate(user.UserID, pair)
					if err == nil {
						h.Value = h.Value.Mul(rate)
					}
				}
			}

			assets := map[string]decimal.Decimal{"stocks": decimal.Zero, "bonds": decimal.Zero, "cash": decimal.Zero, "commodities": decimal.Zero}
			total := decimal.Zero
			for i := range holdings {
				h := &holdings[i]
				assets[h.AssetId] = assets[h.AssetId].Add(h.Value)
				total = total.Add(h.Value)
			}

			var funds []models.AvailableFund
			if err := db.Where("user_id = ? AND portfolio_id = ?", user.UserID, portfolio.ID).Find(&funds).Error; err != nil {
				slog.Error("failed to load available funds for drift test", "error", err)
			}
			for _, f := range funds {
				amt := f.Amount
				if f.Currency != "" && f.Currency != "CNY" {
					pair := f.Currency + "CNY"
					rate, err := router.ExchangeRate(user.UserID, pair)
					if err == nil {
						amt = amt.Mul(rate)
					}
				}
				total = total.Add(amt)
			}

			if total.IsZero() {
				c.JSON(consts.StatusOK, map[string]any{"success": false, "error": "组合无资产数据"})
				return
			}

			settings := make(map[string]string)
			var settingList []models.Setting
			if err := db.Where("portfolio_id = ?", portfolio.ID).Find(&settingList).Error; err == nil {
				for _, s := range settingList {
					settings[s.Key] = s.Value
				}
			}

			targetPcts := map[string]decimal.Decimal{
				"stocks": decimal.NewFromInt(25), "bonds": decimal.NewFromInt(25),
				"cash": decimal.NewFromInt(25), "commodities": decimal.NewFromInt(25),
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

			assetNames := map[string]string{"stocks": "股票", "bonds": "债券", "cash": "现金", "commodities": "商品"}
			lines := []string{"当前资产配置:", ""}
			for _, id := range []string{"stocks", "bonds", "cash", "commodities"} {
				pct := assets[id].Div(total).Mul(decimal.NewFromInt(100))
				diff := pct.Sub(targetPcts[id])
				lines = append(lines, fmt.Sprintf("%s: %s%% (目标 %s%%, 偏离 %s%%)",
					assetNames[id], pct.StringFixed(1), targetPcts[id].StringFixed(0), diff.StringFixed(1)))
			}
			lines = append(lines, "", "— 这是一条测试消息")

			if err := client.SendNotification("配比偏离提醒", strings.Join(lines, "\n"), "配比偏离"); err != nil {
				c.JSON(consts.StatusOK, map[string]any{"success": false, "error": err.Error()})
				return
			}

		case "summary":
			user := middleware.GetUser(c)
			if user == nil {
				c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
				return
			}

			portfolioID, err := uuid.Parse(body.PortfolioID)
			if err != nil {
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的组合ID"})
				return
			}
			owned, err := userOwnsPortfolio(db, user.UserID, portfolioID)
			if err != nil || !owned {
				c.JSON(consts.StatusForbidden, map[string]string{"error": "无权访问该组合"})
				return
			}

			holdings, err := gorm.G[models.Holding](db).Where("user_id = ? AND portfolio_id = ?", user.UserID, portfolioID).Find(ctx)
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

			var funds []models.AvailableFund
			if err := db.Where("user_id = ? AND portfolio_id = ?", user.UserID, portfolioID).Find(&funds).Error; err != nil {
				slog.Error("failed to load available funds for summary test", "error", err)
			}
			for _, f := range funds {
				amt := f.Amount
				if f.Currency != "" && f.Currency != "CNY" {
					pair := f.Currency + "CNY"
					rate, err := router.ExchangeRate(user.UserID, pair)
					if err == nil {
						amt = amt.Mul(rate)
					}
				}
				total = total.Add(amt)
			}

			principal, err := CalcPrincipal(db, portfolioID, "CNY", router)
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
				fmt.Sprintf("总资产: ¥%s", total.StringFixed(2)),
				fmt.Sprintf("累计投入: ¥%s", principal.StringFixed(2)),
			}
			if principal.IsPositive() {
				pnl := total.Sub(principal).Div(principal).Mul(decimal.NewFromInt(100))
				lines = append(lines, fmt.Sprintf("累计收益: %s%%", pnl.StringFixed(2)))
			}
			lines = append(lines, "")

			for _, id := range []string{"stocks", "bonds", "cash", "commodities"} {
				pct := decimal.Zero
				if total.IsPositive() {
					pct = assets[id].Div(total).Mul(decimal.NewFromInt(100))
				}
				lines = append(lines, fmt.Sprintf("%s  %s%%  ¥%s", assetNames[id], pct.StringFixed(1), assets[id].StringFixed(2)))
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
