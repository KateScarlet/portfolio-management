package handlers

import (
	"context"
	"portfolio-management/marketsource"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"time"

	"log/slog"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func ListRecords(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID, err := uuid.Parse(c.Param("pid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的组合ID"})
			return
		}
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

		portfolioID, err := uuid.Parse(c.Param("pid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的组合ID"})
			return
		}
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

		// Snapshot holdings in their original currencies
		snapshotHoldings := make(models.HoldingSnapshotColumn, 0, len(holdings))
		for i := range holdings {
			if holdings[i].Value.IsPositive() {
				snapshotHoldings = append(snapshotHoldings, models.HoldingSnapshot{
					AssetId:   holdings[i].AssetId,
					Symbol:    holdings[i].Symbol,
					Name:      holdings[i].Name,
					Currency:  holdings[i].Currency,
					Shares:    holdings[i].Shares,
					Price:     holdings[i].Price,
					CostPrice: holdings[i].CostPrice,
					Value:     holdings[i].Value,
					Cost:      holdings[i].Cost,
				})
			}
		}

		// Load lots for currency conversion
		holdingIDs := make([]uuid.UUID, len(holdings))
		for i, h := range holdings {
			holdingIDs[i] = h.ID
		}
		lotsMap, err := models.LoadLotsByHoldingIDs(db, holdingIDs)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Deep copy lots for summary computation (convertHoldingsCurrency mutates in-place)
		convertedLotsMap := make(map[uuid.UUID][]models.HoldingLot, len(lotsMap))
		for k, lots := range lotsMap {
			cp := make([]models.HoldingLot, len(lots))
			copy(cp, lots)
			convertedLotsMap[k] = cp
		}

		convertedHoldings := make([]models.Holding, len(holdings))
		copy(convertedHoldings, holdings)
		if err := convertHoldingsCurrency(convertedHoldings, convertedLotsMap, displayCurrency, router, user.UserID); err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		assets := models.AssetMapColumn{"stocks": decimal.Zero, "bonds": decimal.Zero, "cash": decimal.Zero, "commodities": decimal.Zero}
		total := decimal.Zero
		for i := range convertedHoldings {
			assets[convertedHoldings[i].AssetId] = assets[convertedHoldings[i].AssetId].Add(convertedHoldings[i].Value)
			total = total.Add(convertedHoldings[i].Value)
		}

		if total.IsZero() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "没有数据可记录"})
			return
		}

		principal, err := CalcPrincipal(db, portfolioID, displayCurrency, router)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		record := models.PortfolioRecord{
			ID:          uuid.New(),
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

		portfolioID, err := uuid.Parse(c.Param("pid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的组合ID"})
			return
		}
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

		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的记录ID"})
			return
		}
		rows, err := gorm.G[models.PortfolioRecord](db).Where("portfolio_id = ? AND id = ?", portfolioID, id).Delete(ctx)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if rows == 0 {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "记录不存在"})
			return
		}
		c.JSON(consts.StatusOK, map[string]bool{"success": true})
	}
}
