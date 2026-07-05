package handlers

import (
	"context"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type RecordDividendRequest struct {
	HoldingID        string          `json:"holdingId" binding:"required"`
	Amount           decimal.Decimal `json:"amount" binding:"required"`
	TaxWithheld      decimal.Decimal `json:"taxWithheld"`
	Currency         string          `json:"currency"`
	DividendPerShare decimal.Decimal `json:"dividendPerShare"`
	ExDate           int64           `json:"exDate"`
	PayDate          int64           `json:"payDate"`
	Reinvest         bool            `json:"reinvest"`
	ReinvestPrice    decimal.Decimal `json:"reinvestPrice"`
	Note             string          `json:"note"`
}

func RecordDividend(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID := c.Param("pid")

		var req RecordDividendRequest
		if err := c.BindAndValidate(&req); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		var portfolio models.Portfolio
		if err := db.Where("id = ? AND user_id = ?", portfolioID, user.UserID).First(&portfolio).Error; err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "投资组合不存在"})
			return
		}

		var holding models.Holding
		if err := db.Where("id = ? AND portfolio_id = ? AND user_id = ?", req.HoldingID, portfolioID, user.UserID).First(&holding).Error; err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "持仓不存在"})
			return
		}

		netAmount := req.Amount.Sub(req.TaxWithheld)
		if netAmount.LessThanOrEqual(decimal.Zero) {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "净分红金额必须大于0"})
			return
		}

		if req.Reinvest && req.ReinvestPrice.LessThanOrEqual(decimal.Zero) {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "再投资价格必须大于0"})
			return
		}

		currency := req.Currency
		if currency == "" {
			currency = holding.Currency
		}

		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		dividend := models.Dividend{
			ID:               uuid.New().String(),
			UserID:           user.UserID,
			PortfolioID:      portfolioID,
			HoldingID:        req.HoldingID,
			AssetId:          holding.AssetId,
			Symbol:           holding.Symbol,
			Amount:           req.Amount,
			TaxWithheld:      req.TaxWithheld,
			NetAmount:        netAmount,
			Currency:         currency,
			SharesHeld:       holding.Shares,
			DividendPerShare: req.DividendPerShare,
			ExDate:           req.ExDate,
			PayDate:          req.PayDate,
			Reinvest:         req.Reinvest,
			ReinvestPrice:    req.ReinvestPrice,
			Note:             req.Note,
			CreatedAt:        time.Now().UnixMilli(),
		}

		if err := tx.Create(&dividend).Error; err != nil {
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		holding.TotalDividends = holding.TotalDividends.Add(netAmount)
		if err := tx.Save(&holding).Error; err != nil {
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if req.Reinvest {
			reinvestShares := netAmount.Div(req.ReinvestPrice)

			lot := models.HoldingLot{
				ID:        uuid.New().String(),
				Type:      "buy",
				Date:      time.Now().UnixMilli(),
				Shares:    reinvestShares,
				CostPrice: req.ReinvestPrice,
				Cost:      netAmount,
				ValueAdded: netAmount,
				Fee:       decimal.Zero,
			}

			holding.Lots = append(holding.Lots, lot)
			holding.RecalcFromLots()
			if err := tx.Save(&holding).Error; err != nil {
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			dividend.HoldingLotID = lot.ID
			if err := tx.Save(&dividend).Error; err != nil {
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			fundTx := models.FundTransaction{
				ID:          uuid.New().String(),
				UserID:      user.UserID,
				PortfolioID: portfolioID,
				Type:        "dividend_reinvest",
				Amount:      netAmount,
				Currency:    currency,
				HoldingID:   req.HoldingID,
				Note:        req.Note,
				CreatedAt:   time.Now().UnixMilli(),
			}
			if err := tx.Create(&fundTx).Error; err != nil {
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			dividend.FundTxID = fundTx.ID
			if err := tx.Save(&dividend).Error; err != nil {
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		} else {
			if err := addAvailableFund(tx, user.UserID, portfolioID, currency, netAmount); err != nil {
				tx.Rollback()
				if httpErr, ok := err.(*httpError); ok {
					c.JSON(httpErr.status, map[string]string{"error": httpErr.msg})
				} else {
					c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				}
				return
			}

			fundTx := models.FundTransaction{
				ID:          uuid.New().String(),
				UserID:      user.UserID,
				PortfolioID: portfolioID,
				Type:        "dividend",
				Amount:      netAmount,
				Currency:    currency,
				HoldingID:   req.HoldingID,
				Note:        req.Note,
				CreatedAt:   time.Now().UnixMilli(),
			}
			if err := tx.Create(&fundTx).Error; err != nil {
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			dividend.FundTxID = fundTx.ID
			if err := tx.Save(&dividend).Error; err != nil {
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}

		if err := tx.Commit().Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusCreated, dividend)
	}
}

func ListDividends(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID := c.Param("pid")

		var portfolio models.Portfolio
		if err := db.Where("id = ? AND user_id = ?", portfolioID, user.UserID).First(&portfolio).Error; err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "投资组合不存在"})
			return
		}

		query := db.Where("portfolio_id = ? AND user_id = ?", portfolioID, user.UserID)

		holdingID := c.Query("holdingId")
		if holdingID != "" {
			query = query.Where("holding_id = ?", holdingID)
		}

		var dividends []models.Dividend
		if err := query.Order("created_at DESC").Find(&dividends).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusOK, dividends)
	}
}

func DeleteDividend(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID := c.Param("pid")
		dividendID := c.Param("id")

		var dividend models.Dividend
		if err := db.Where("id = ? AND portfolio_id = ? AND user_id = ?", dividendID, portfolioID, user.UserID).First(&dividend).Error; err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "分红记录不存在"})
			return
		}

		tx := db.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		if dividend.Reinvest && dividend.HoldingLotID != "" {
			var holding models.Holding
			if err := tx.Where("id = ? AND user_id = ?", dividend.HoldingID, user.UserID).First(&holding).Error; err != nil {
				tx.Rollback()
				c.JSON(consts.StatusNotFound, map[string]string{"error": "持仓不存在"})
				return
			}

			var updatedLots []models.HoldingLot
			for _, lot := range holding.Lots {
				if lot.ID != dividend.HoldingLotID {
					updatedLots = append(updatedLots, lot)
				}
			}
			holding.Lots = updatedLots
			holding.RecalcFromLots()
			if err := tx.Save(&holding).Error; err != nil {
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		} else {
			if err := deductAvailableFund(tx, user.UserID, portfolioID, dividend.Currency, dividend.NetAmount); err != nil {
				tx.Rollback()
				if httpErr, ok := err.(*httpError); ok {
					c.JSON(httpErr.status, map[string]string{"error": httpErr.msg})
				} else {
					c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				}
				return
			}
		}

		var holding models.Holding
		if err := tx.Where("id = ? AND user_id = ?", dividend.HoldingID, user.UserID).First(&holding).Error; err == nil {
			holding.TotalDividends = holding.TotalDividends.Sub(dividend.NetAmount)
			tx.Save(&holding)
		}

		if dividend.FundTxID != "" {
			tx.Where("id = ?", dividend.FundTxID).Delete(&models.FundTransaction{})
		}

		if err := tx.Delete(&dividend).Error; err != nil {
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if err := tx.Commit().Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusOK, map[string]string{"status": "ok"})
	}
}