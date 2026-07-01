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

		var records []models.PortfolioRecord
		if err := db.Where("portfolio_id = ?", portfolioID).Order("timestamp DESC").Find(&records).Error; err != nil {
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

		var holdings []models.Holding
		if err := db.Where("portfolio_id = ?", portfolioID).Find(&holdings).Error; err != nil {
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

		assets := models.AssetMapColumn{"stocks": 0, "bonds": 0, "cash": 0, "commodities": 0}
		var total float64
		snapshotHoldings := make(models.HoldingSnapshotColumn, 0, len(holdings))
		for i := range holdings {
			assets[holdings[i].AssetId] += holdings[i].Value
			total += holdings[i].Value

			if holdings[i].Value > 0 {
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

		if total == 0 {
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

		if err := db.Create(&record).Error; err != nil {
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
		result := db.Where("portfolio_id = ?", portfolioID).Delete(&models.PortfolioRecord{}, "id = ?", id)
		if result.Error != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": result.Error.Error()})
			return
		}
		if result.RowsAffected == 0 {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "Record not found"})
			return
		}
		c.JSON(consts.StatusOK, map[string]bool{"success": true})
	}
}
