package handlers

import (
	"context"
	"portfolio-management/marketsource"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"log/slog"
)

func ListRecords(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID := c.Param("pid")
		owns, err := userOwnsPortfolio(db, user.UserID, portfolioID)
		if err != nil {
			slog.Error("failed to check portfolio ownership", "error", err)
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "数据库错误"})
			return
		}
		if !owns {
			c.JSON(consts.StatusForbidden, map[string]string{"error": "无权访问此组合"})
			return
		}

		records, err := gorm.G[models.PortfolioRecord](db).Where("portfolio_id = ?", portfolioID).Order("timestamp DESC").Find(ctx)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(consts.StatusOK, records)
	}
}

func CreateRecord(db *gorm.DB, router *marketsource.Router) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID := c.Param("pid")
		owns, err := userOwnsPortfolio(db, user.UserID, portfolioID)
		if err != nil {
			slog.Error("failed to check portfolio ownership", "error", err)
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "数据库错误"})
			return
		}
		if !owns {
			c.JSON(consts.StatusForbidden, map[string]string{"error": "无权访问此组合"})
			return
		}

		holdings, err := gorm.G[models.Holding](db).Where("portfolio_id = ?", portfolioID).Find(ctx)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		displayCurrency := c.Query("currency")
		if displayCurrency == "" {
			displayCurrency = "CNY"
		}
		if err := convertHoldingsCurrency(holdings, displayCurrency, router, user.UserID); err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		assets := models.AssetMapColumn{"stocks": decimal.Zero, "bonds": decimal.Zero, "cash": decimal.Zero, "commodities": decimal.Zero}
		total := decimal.Zero
		snapshotHoldings := make(models.HoldingSnapshotColumn, 0, len(holdings))
		for i := range holdings {
			assets[holdings[i].AssetId] = assets[holdings[i].AssetId].Add(holdings[i].Value)
			total = total.Add(holdings[i].Value)

			if holdings[i].Value.IsPositive() {
				snapshotHoldings = append(snapshotHoldings, models.HoldingSnapshot{
					AssetId:   holdings[i].AssetId,
					Symbol:    holdings[i].Symbol,
					Name:      holdings[i].Name,
					Currency:  displayCurrency,
					Shares:    holdings[i].Shares,
					Price:     holdings[i].Price,
					CostPrice: holdings[i].CostPrice,
					Value:     holdings[i].Value,
					Cost:      holdings[i].Cost,
				})
			}
		}

		if total.IsZero() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "No data to record"})
			return
		}

		principal, err := CalcPrincipal(db, portfolioID, displayCurrency, router)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		record := models.PortfolioRecord{
			ID:          uuid.New().String(),
			UserID:      user.UserID,
			PortfolioID: portfolioID,
			Timestamp:   time.Now().UnixMilli(),
			Assets:      assets,
			Holdings:    snapshotHoldings,
			Total:       total,
			Principal:   principal,
		}

		if err := gorm.G[models.PortfolioRecord](db).Create(ctx, &record); err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusCreated, record)
	}
}

func DeleteRecord(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID := c.Param("pid")
		owns, err := userOwnsPortfolio(db, user.UserID, portfolioID)
		if err != nil {
			slog.Error("failed to check portfolio ownership", "error", err)
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "数据库错误"})
			return
		}
		if !owns {
			c.JSON(consts.StatusForbidden, map[string]string{"error": "无权访问此组合"})
			return
		}

		id := c.Param("id")
		rows, err := gorm.G[models.PortfolioRecord](db).Where("portfolio_id = ? AND id = ?", portfolioID, id).Delete(ctx)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if rows == 0 {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "Record not found"})
			return
		}
		c.JSON(consts.StatusOK, map[string]bool{"success": true})
	}
}
