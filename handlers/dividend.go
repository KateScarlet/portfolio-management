package handlers

import (
	"context"
	"errors"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type UpdateDividendRequest struct {
	Amount           decimal.Decimal `json:"amount" binding:"required"`
	TaxWithheld      decimal.Decimal `json:"taxWithheld"`
	Currency         string          `json:"currency"`
	DividendPerShare decimal.Decimal `json:"dividendPerShare"`
	ExDate           time.Time       `json:"exDate"`
	PayDate          time.Time       `json:"payDate"`
	Reinvest         bool            `json:"reinvest"`
	ReinvestPrice    decimal.Decimal `json:"reinvestPrice"`
	Note             string          `json:"note"`
}

type RecordDividendRequest struct {
	HoldingID        uuid.UUID       `json:"holdingId" binding:"required"`
	Amount           decimal.Decimal `json:"amount" binding:"required"`
	TaxWithheld      decimal.Decimal `json:"taxWithheld"`
	Currency         string          `json:"currency"`
	DividendPerShare decimal.Decimal `json:"dividendPerShare"`
	ExDate           time.Time       `json:"exDate"`
	PayDate          time.Time       `json:"payDate"`
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

		portfolioID, err := uuid.Parse(c.Param("pid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的投资组合ID"})
			return
		}

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

		var dividend models.Dividend

		err = db.Transaction(func(tx *gorm.DB) error {
			dividend = models.Dividend{
				ID:               uuid.New(),
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
			}

			if err := tx.Create(&dividend).Error; err != nil {
				return err
			}

			holding.TotalDividends = holding.TotalDividends.Add(netAmount)
			if err := tx.Save(&holding).Error; err != nil {
				return err
			}

			if req.Reinvest {
				reinvestShares := netAmount.Div(req.ReinvestPrice)

				lot := models.HoldingLot{
					ID:         uuid.New(),
					HoldingID:  holding.ID,
					Type:       "buy",
					Date:       time.Now(),
					Shares:     reinvestShares,
					CostPrice:  req.ReinvestPrice,
					Cost:       netAmount,
					ValueAdded: netAmount,
					Fee:        decimal.Zero,
				}

				if err := models.CreateLot(tx, &lot); err != nil {
					return err
				}

				lots, err := models.LoadLots(tx, holding.ID)
				if err != nil {
					return err
				}
				models.RecalcFromLots(&holding, lots)
				if err := tx.Save(&holding).Error; err != nil {
					return err
				}

				dividend.HoldingLotID = &lot.ID
				if err := tx.Save(&dividend).Error; err != nil {
					return err
				}

				fundTx := models.FundTransaction{
					ID:          uuid.New(),
					UserID:      user.UserID,
					PortfolioID: portfolioID,
					Type:        "dividend_reinvest",
					Amount:      netAmount,
					Currency:    currency,
					HoldingID:   &req.HoldingID,
					Note:        req.Note,
				}
				if err := tx.Create(&fundTx).Error; err != nil {
					return err
				}

				dividend.FundTxID = &fundTx.ID
				if err := tx.Save(&dividend).Error; err != nil {
					return err
				}
			} else {
				if err := addAvailableFund(tx, user.UserID, portfolioID, currency, netAmount); err != nil {
					return err
				}

				fundTx := models.FundTransaction{
					ID:          uuid.New(),
					UserID:      user.UserID,
					PortfolioID: portfolioID,
					Type:        "dividend",
					Amount:      netAmount,
					Currency:    currency,
					HoldingID:   &req.HoldingID,
					Note:        req.Note,
				}
				if err := tx.Create(&fundTx).Error; err != nil {
					return err
				}

				dividend.FundTxID = &fundTx.ID
				if err := tx.Save(&dividend).Error; err != nil {
					return err
				}
			}

			return nil
		})
		if err != nil {
			if httpErr, ok := errors.AsType[*httpError](err); ok {
				c.JSON(httpErr.status, map[string]string{"error": httpErr.msg})
			} else {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
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

		portfolioID, err := uuid.Parse(c.Param("pid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的投资组合ID"})
			return
		}

		var portfolio models.Portfolio
		if err := db.Where("id = ? AND user_id = ?", portfolioID, user.UserID).First(&portfolio).Error; err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "投资组合不存在"})
			return
		}

		query := db.Where("portfolio_id = ? AND user_id = ?", portfolioID, user.UserID)

		holdingIDStr := c.Query("holdingId")
		if holdingIDStr != "" {
			holdingID, err := uuid.Parse(holdingIDStr)
			if err != nil {
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的持仓ID"})
				return
			}
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

		portfolioID, err := uuid.Parse(c.Param("pid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的投资组合ID"})
			return
		}
		dividendID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的分红记录ID"})
			return
		}

		var dividend models.Dividend
		if err := db.Where("id = ? AND portfolio_id = ? AND user_id = ?", dividendID, portfolioID, user.UserID).First(&dividend).Error; err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "分红记录不存在"})
			return
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			if dividend.Reinvest && dividend.HoldingLotID != nil {
				if err := models.DeleteLotByID(tx, *dividend.HoldingLotID); err != nil {
					return err
				}

				lots, err := models.LoadLots(tx, dividend.HoldingID)
				if err != nil {
					return err
				}
				var holding models.Holding
				if err := tx.Where("id = ? AND user_id = ?", dividend.HoldingID, user.UserID).First(&holding).Error; err != nil {
					return &httpError{status: consts.StatusNotFound, msg: "持仓不存在"}
				}
				models.RecalcFromLots(&holding, lots)
				if err := tx.Save(&holding).Error; err != nil {
					return err
				}
			} else {
				if err := deductAvailableFund(tx, user.UserID, portfolioID, dividend.Currency, dividend.NetAmount); err != nil {
					return err
				}
			}

			var holding models.Holding
			if err := tx.Where("id = ? AND user_id = ?", dividend.HoldingID, user.UserID).First(&holding).Error; err == nil {
				holding.TotalDividends = holding.TotalDividends.Sub(dividend.NetAmount)
				if err := tx.Save(&holding).Error; err != nil {
					return err
				}
			}

			if dividend.FundTxID != nil {
				if err := tx.Where("id = ?", *dividend.FundTxID).Delete(&models.FundTransaction{}).Error; err != nil {
					return err
				}
			}

			if err := tx.Delete(&dividend).Error; err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			if httpErr, ok := errors.AsType[*httpError](err); ok {
				c.JSON(httpErr.status, map[string]string{"error": httpErr.msg})
			} else {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}

		c.JSON(consts.StatusOK, map[string]string{"status": "ok"})
	}
}

func UpdateDividend(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID, err := uuid.Parse(c.Param("pid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的投资组合ID"})
			return
		}
		dividendID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的分红记录ID"})
			return
		}

		var req UpdateDividendRequest
		if err := c.BindAndValidate(&req); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		var dividend models.Dividend
		if err := db.Where("id = ? AND portfolio_id = ? AND user_id = ?", dividendID, portfolioID, user.UserID).First(&dividend).Error; err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "分红记录不存在"})
			return
		}

		var holding models.Holding
		if err := db.Where("id = ? AND portfolio_id = ? AND user_id = ?", dividend.HoldingID, portfolioID, user.UserID).First(&holding).Error; err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "持仓不存在"})
			return
		}

		newNetAmount := req.Amount.Sub(req.TaxWithheld)
		if newNetAmount.LessThanOrEqual(decimal.Zero) {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "净分红金额必须大于0"})
			return
		}

		if req.Reinvest && req.ReinvestPrice.LessThanOrEqual(decimal.Zero) {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "再投资价格必须大于0"})
			return
		}

		currency := req.Currency
		if currency == "" {
			currency = dividend.Currency
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			// 1. Reverse old side effects
			if dividend.Reinvest && dividend.HoldingLotID != nil {
				if err := models.DeleteLotByID(tx, *dividend.HoldingLotID); err != nil {
					return err
				}

				lots, err := models.LoadLots(tx, dividend.HoldingID)
				if err != nil {
					return err
				}
				models.RecalcFromLots(&holding, lots)
				if err := tx.Save(&holding).Error; err != nil {
					return err
				}
			} else {
				if err := deductAvailableFund(tx, user.UserID, portfolioID, dividend.Currency, dividend.NetAmount); err != nil {
					return err
				}
			}

			holding.TotalDividends = holding.TotalDividends.Sub(dividend.NetAmount)
			if err := tx.Save(&holding).Error; err != nil {
				return err
			}

			if dividend.FundTxID != nil {
				if err := tx.Where("id = ?", *dividend.FundTxID).Delete(&models.FundTransaction{}).Error; err != nil {
					return err
				}
			}

			// 2. Apply new side effects
			holding.TotalDividends = holding.TotalDividends.Add(newNetAmount)

			if req.Reinvest {
				reinvestShares := newNetAmount.Div(req.ReinvestPrice)

				lot := models.HoldingLot{
					ID:         uuid.New(),
					HoldingID:  holding.ID,
					Type:       "buy",
					Date:       time.Now(),
					Shares:     reinvestShares,
					CostPrice:  req.ReinvestPrice,
					Cost:       newNetAmount,
					ValueAdded: newNetAmount,
					Fee:        decimal.Zero,
				}

				if err := models.CreateLot(tx, &lot); err != nil {
					return err
				}

				lots, err := models.LoadLots(tx, holding.ID)
				if err != nil {
					return err
				}
				models.RecalcFromLots(&holding, lots)
				if err := tx.Save(&holding).Error; err != nil {
					return err
				}

				dividend.HoldingLotID = &lot.ID

				fundTx := models.FundTransaction{
					ID:          uuid.New(),
					UserID:      user.UserID,
					PortfolioID: portfolioID,
					Type:        "dividend_reinvest",
					Amount:      newNetAmount,
					Currency:    currency,
					HoldingID:   &dividend.HoldingID,
					Note:        req.Note,
				}
				if err := tx.Create(&fundTx).Error; err != nil {
					return err
				}
				dividend.FundTxID = &fundTx.ID
			} else {
				if err := addAvailableFund(tx, user.UserID, portfolioID, currency, newNetAmount); err != nil {
					return err
				}

				fundTx := models.FundTransaction{
					ID:          uuid.New(),
					UserID:      user.UserID,
					PortfolioID: portfolioID,
					Type:        "dividend",
					Amount:      newNetAmount,
					Currency:    currency,
					HoldingID:   &dividend.HoldingID,
					Note:        req.Note,
				}
				if err := tx.Create(&fundTx).Error; err != nil {
					return err
				}
				dividend.FundTxID = &fundTx.ID
			}

			// 3. Update dividend record fields
			dividend.Amount = req.Amount
			dividend.TaxWithheld = req.TaxWithheld
			dividend.NetAmount = newNetAmount
			dividend.Currency = currency
			dividend.DividendPerShare = req.DividendPerShare
			dividend.ExDate = req.ExDate
			dividend.PayDate = req.PayDate
			dividend.Reinvest = req.Reinvest
			dividend.ReinvestPrice = req.ReinvestPrice
			dividend.Note = req.Note

			if err := tx.Save(&dividend).Error; err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			if httpErr, ok := errors.AsType[*httpError](err); ok {
				c.JSON(httpErr.status, map[string]string{"error": httpErr.msg})
			} else {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}

		c.JSON(consts.StatusOK, dividend)
	}
}
