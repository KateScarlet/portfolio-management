package handlers

import (
	"context"
	"errors"
	"strings"
	"time"
	"uuid"

	"portfolio-management/middleware"
	"portfolio-management/models"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	DividendTypeCash     = "cash"
	DividendTypeReinvest = "reinvest"
)

type CreateDividendRequest struct {
	HoldingID         uuid.UUID       `json:"holdingId"`
	GrossAmount       decimal.Decimal `json:"grossAmount"`
	TaxAmount         decimal.Decimal `json:"taxAmount"`
	Type              string          `json:"type"`
	PaymentDate       time.Time       `json:"paymentDate"`
	ReinvestmentPrice decimal.Decimal `json:"reinvestmentPrice"`
	Note              string          `json:"note"`
}

type UpdateDividendRequest struct {
	GrossAmount       decimal.Decimal `json:"grossAmount"`
	TaxAmount         decimal.Decimal `json:"taxAmount"`
	Type              string          `json:"type"`
	PaymentDate       time.Time       `json:"paymentDate"`
	ReinvestmentPrice decimal.Decimal `json:"reinvestmentPrice"`
	Note              string          `json:"note"`
}

type dividendInput struct {
	GrossAmount       decimal.Decimal
	TaxAmount         decimal.Decimal
	Type              string
	PaymentDate       time.Time
	ReinvestmentPrice decimal.Decimal
	Note              string
}

func validateDividendInput(input dividendInput) (decimal.Decimal, *httpError) {
	input.Type = strings.ToLower(strings.TrimSpace(input.Type))
	if !input.GrossAmount.IsPositive() {
		return decimal.Zero, &httpError{status: consts.StatusBadRequest, msg: "分红总额必须大于0"}
	}
	if input.TaxAmount.IsNegative() {
		return decimal.Zero, &httpError{status: consts.StatusBadRequest, msg: "预扣税不能为负数"}
	}
	if input.TaxAmount.GreaterThanOrEqual(input.GrossAmount) {
		return decimal.Zero, &httpError{status: consts.StatusBadRequest, msg: "预扣税必须小于分红总额"}
	}
	if input.Type != DividendTypeCash && input.Type != DividendTypeReinvest {
		return decimal.Zero, &httpError{status: consts.StatusBadRequest, msg: "分红类型必须是 cash 或 reinvest"}
	}
	if input.PaymentDate.IsZero() {
		return decimal.Zero, &httpError{status: consts.StatusBadRequest, msg: "支付日期不能为空"}
	}
	if input.Type == DividendTypeReinvest && !input.ReinvestmentPrice.IsPositive() {
		return decimal.Zero, &httpError{status: consts.StatusBadRequest, msg: "再投资价格必须大于0"}
	}
	if len(input.Note) > 500 {
		return decimal.Zero, &httpError{status: consts.StatusBadRequest, msg: "备注不能超过500个字符"}
	}
	return input.GrossAmount.Sub(input.TaxAmount), nil
}

func writeDividendError(c *app.RequestContext, err error) {
	if he, ok := errors.AsType[*httpError](err); ok {
		c.JSON(he.status, map[string]string{"error": he.msg})
		return
	}
	c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
}

func parsePortfolioAndUser(c *app.RequestContext) (*middleware.JWTClaims, uuid.UUID, *httpError) {
	user := middleware.GetUser(c)
	if user == nil {
		return nil, uuid.Nil(), &httpError{status: consts.StatusUnauthorized, msg: "未登录"}
	}
	portfolioID, err := uuid.Parse(c.Param("pid"))
	if err != nil {
		return nil, uuid.Nil(), &httpError{status: consts.StatusBadRequest, msg: "无效的投资组合ID"}
	}
	return user, portfolioID, nil
}

func lockHolding(tx *gorm.DB, userID, portfolioID, holdingID uuid.UUID) (models.Holding, error) {
	var holding models.Holding
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND portfolio_id = ? AND user_id = ?", holdingID, portfolioID, userID).
		First(&holding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return holding, &httpError{status: consts.StatusNotFound, msg: "持仓不存在"}
	}
	return holding, err
}

func changeAvailableFund(tx *gorm.DB, userID, portfolioID uuid.UUID, currency string, delta decimal.Decimal) error {
	if delta.IsZero() {
		return nil
	}
	if delta.IsPositive() {
		return addAvailableFund(tx, userID, portfolioID, currency, delta)
	}
	if err := deductAvailableFund(tx, userID, portfolioID, currency, delta.Abs()); err != nil {
		if _, ok := errors.AsType[*httpError](err); ok {
			return &httpError{status: consts.StatusBadRequest, msg: "可用资金不足，无法撤销该分红"}
		}
		return err
	}
	return nil
}

func ensureReinvestmentReversible(tx *gorm.DB, dividend models.Dividend) error {
	if dividend.Type != DividendTypeReinvest || dividend.HoldingLotID == nil {
		return nil
	}
	var count int64
	if err := tx.Model(&models.HoldingLot{}).
		Where("holding_id = ? AND type = ? AND date >= ?", dividend.HoldingID, "sell", dividend.PaymentDate).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return &httpError{status: consts.StatusConflict, msg: "该再投资分红之后已有卖出记录，不能修改或删除"}
	}
	return nil
}

func ensureNoSalesSince(tx *gorm.DB, holdingID uuid.UUID, date time.Time) error {
	var count int64
	if err := tx.Model(&models.HoldingLot{}).
		Where("holding_id = ? AND type = ? AND date >= ?", holdingID, "sell", date).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return &httpError{status: consts.StatusConflict, msg: "支付日期之后已有卖出记录，不能新增或改为再投资分红"}
	}
	return nil
}

func createDividendLot(holding models.Holding, paymentDate time.Time, netAmount, price decimal.Decimal) models.HoldingLot {
	return models.HoldingLot{
		ID: uuid.New(), HoldingID: holding.ID, Type: "buy", Date: paymentDate,
		Shares: netAmount.Div(price), CostPrice: price, Cost: netAmount,
		ValueAdded: netAmount, Fee: decimal.Zero, Source: "dividend_reinvest",
	}
}

func RecordDividend(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user, portfolioID, he := parsePortfolioAndUser(c)
		if he != nil {
			writeDividendError(c, he)
			return
		}

		var req CreateDividendRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		input := dividendInput{req.GrossAmount, req.TaxAmount, strings.ToLower(strings.TrimSpace(req.Type)), req.PaymentDate, req.ReinvestmentPrice, strings.TrimSpace(req.Note)}
		netAmount, he := validateDividendInput(input)
		if he != nil {
			writeDividendError(c, he)
			return
		}
		if req.HoldingID == uuid.Nil() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "持仓ID不能为空"})
			return
		}

		var dividend models.Dividend
		err := db.Transaction(func(tx *gorm.DB) error {
			holding, err := lockHolding(tx, user.UserID, portfolioID, req.HoldingID)
			if err != nil {
				return err
			}
			currency := holding.Currency
			if currency == "" {
				currency = "CNY"
			}
			fundTxID := uuid.New()
			dividend = models.Dividend{
				ID: uuid.New(), UserID: user.UserID, PortfolioID: portfolioID, HoldingID: holding.ID,
				Type: input.Type, GrossAmount: input.GrossAmount, TaxAmount: input.TaxAmount, NetAmount: netAmount,
				Currency: currency, PaymentDate: input.PaymentDate, SharesAtPayment: holding.Shares,
				ReinvestmentPrice: input.ReinvestmentPrice, FundTxID: fundTxID, Note: input.Note,
			}
			if input.Type == DividendTypeCash {
				dividend.ReinvestmentPrice = decimal.Zero
				if err := changeAvailableFund(tx, user.UserID, portfolioID, currency, netAmount); err != nil {
					return err
				}
			} else {
				if err := ensureNoSalesSince(tx, holding.ID, input.PaymentDate); err != nil {
					return err
				}
				lot := createDividendLot(holding, input.PaymentDate, netAmount, input.ReinvestmentPrice)
				if err := models.CreateLot(tx, &lot); err != nil {
					return err
				}
				dividend.HoldingLotID = &lot.ID
				dividend.ReinvestedShares = lot.Shares
			}
			holding.TotalDividends = holding.TotalDividends.Add(netAmount)
			lots, err := models.LoadLots(tx, holding.ID)
			if err != nil {
				return err
			}
			models.RecalcFromLots(&holding, lots)
			if err := tx.Model(&holding).Updates(map[string]any{
				"shares": holding.Shares, "value": holding.Value, "cost": holding.Cost,
				"cost_price": holding.CostPrice, "total_dividends": holding.TotalDividends,
			}).Error; err != nil {
				return err
			}
			if err := tx.Create(&models.FundTransaction{
				ID: fundTxID, UserID: user.UserID, PortfolioID: portfolioID,
				Type: "dividend_" + input.Type, Amount: netAmount, Currency: currency, HoldingID: &holding.ID, Note: input.Note,
			}).Error; err != nil {
				return err
			}
			return tx.Create(&dividend).Error
		})
		if err != nil {
			writeDividendError(c, err)
			return
		}
		c.JSON(consts.StatusCreated, dividend)
	}
}

func ListDividends(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user, portfolioID, he := parsePortfolioAndUser(c)
		if he != nil {
			writeDividendError(c, he)
			return
		}
		query := db.Where("portfolio_id = ? AND user_id = ?", portfolioID, user.UserID)
		if value := c.Query("holdingId"); value != "" {
			holdingID, err := uuid.Parse(value)
			if err != nil {
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的持仓ID"})
				return
			}
			query = query.Where("holding_id = ?", holdingID)
		}
		var dividends []models.Dividend
		if err := query.Order("payment_date DESC, created_at DESC").Find(&dividends).Error; err != nil {
			writeDividendError(c, err)
			return
		}
		c.JSON(consts.StatusOK, dividends)
	}
}

func UpdateDividend(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user, portfolioID, he := parsePortfolioAndUser(c)
		if he != nil {
			writeDividendError(c, he)
			return
		}
		dividendID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的分红记录ID"})
			return
		}
		var req UpdateDividendRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		input := dividendInput{req.GrossAmount, req.TaxAmount, strings.ToLower(strings.TrimSpace(req.Type)), req.PaymentDate, req.ReinvestmentPrice, strings.TrimSpace(req.Note)}
		newNetAmount, he := validateDividendInput(input)
		if he != nil {
			writeDividendError(c, he)
			return
		}

		var dividend models.Dividend
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND portfolio_id = ? AND user_id = ?", dividendID, portfolioID, user.UserID).First(&dividend).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return &httpError{status: consts.StatusNotFound, msg: "分红记录不存在"}
				}
				return err
			}
			holding, err := lockHolding(tx, user.UserID, portfolioID, dividend.HoldingID)
			if err != nil {
				return err
			}
			if err := ensureReinvestmentReversible(tx, dividend); err != nil {
				return err
			}

			cashDelta := decimal.Zero
			if dividend.Type == DividendTypeCash {
				cashDelta = cashDelta.Sub(dividend.NetAmount)
			}
			if input.Type == DividendTypeCash {
				cashDelta = cashDelta.Add(newNetAmount)
			}
			if err := changeAvailableFund(tx, user.UserID, portfolioID, dividend.Currency, cashDelta); err != nil {
				return err
			}

			if dividend.HoldingLotID != nil {
				if err := models.DeleteLotByID(tx, *dividend.HoldingLotID); err != nil {
					return err
				}
			}
			dividend.HoldingLotID = nil
			dividend.ReinvestedShares = decimal.Zero
			if input.Type == DividendTypeReinvest {
				if err := ensureNoSalesSince(tx, holding.ID, input.PaymentDate); err != nil {
					return err
				}
				lot := createDividendLot(holding, input.PaymentDate, newNetAmount, input.ReinvestmentPrice)
				if err := models.CreateLot(tx, &lot); err != nil {
					return err
				}
				dividend.HoldingLotID = &lot.ID
				dividend.ReinvestedShares = lot.Shares
			}
			lots, err := models.LoadLots(tx, holding.ID)
			if err != nil {
				return err
			}
			holding.TotalDividends = holding.TotalDividends.Add(newNetAmount.Sub(dividend.NetAmount))
			models.RecalcFromLots(&holding, lots)
			if err := tx.Model(&holding).Updates(map[string]any{
				"shares": holding.Shares, "value": holding.Value, "cost": holding.Cost, "cost_price": holding.CostPrice,
				"total_dividends": holding.TotalDividends,
			}).Error; err != nil {
				return err
			}

			if err := tx.Where("id = ? AND user_id = ?", dividend.FundTxID, user.UserID).Delete(&models.FundTransaction{}).Error; err != nil {
				return err
			}
			fundTxID := uuid.New()
			if err := tx.Create(&models.FundTransaction{ID: fundTxID, UserID: user.UserID, PortfolioID: portfolioID, Type: "dividend_" + input.Type, Amount: newNetAmount, Currency: dividend.Currency, HoldingID: &holding.ID, Note: input.Note}).Error; err != nil {
				return err
			}

			dividend.Type = input.Type
			dividend.GrossAmount = input.GrossAmount
			dividend.TaxAmount = input.TaxAmount
			dividend.NetAmount = newNetAmount
			dividend.PaymentDate = input.PaymentDate
			dividend.ReinvestmentPrice = decimal.Zero
			if input.Type == DividendTypeReinvest {
				dividend.ReinvestmentPrice = input.ReinvestmentPrice
			}
			dividend.FundTxID = fundTxID
			dividend.Note = input.Note
			if err := tx.Save(&dividend).Error; err != nil {
				return err
			}
			// GORM may omit nil pointer fields when saving the struct. Explicitly
			// clear the old reinvestment lot reference when converting to cash.
			if input.Type == DividendTypeCash {
				if err := tx.Model(&dividend).Update("holding_lot_id", nil).Error; err != nil {
					return err
				}
				dividend.HoldingLotID = nil
			}
			return nil
		})
		if err != nil {
			writeDividendError(c, err)
			return
		}
		c.JSON(consts.StatusOK, dividend)
	}
}

func DeleteDividend(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user, portfolioID, he := parsePortfolioAndUser(c)
		if he != nil {
			writeDividendError(c, he)
			return
		}
		dividendID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的分红记录ID"})
			return
		}
		err = db.Transaction(func(tx *gorm.DB) error {
			var dividend models.Dividend
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND portfolio_id = ? AND user_id = ?", dividendID, portfolioID, user.UserID).First(&dividend).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return &httpError{status: consts.StatusNotFound, msg: "分红记录不存在"}
				}
				return err
			}
			holding, err := lockHolding(tx, user.UserID, portfolioID, dividend.HoldingID)
			if err != nil {
				return err
			}
			if err := ensureReinvestmentReversible(tx, dividend); err != nil {
				return err
			}
			if dividend.Type == DividendTypeCash {
				if err := changeAvailableFund(tx, user.UserID, portfolioID, dividend.Currency, dividend.NetAmount.Neg()); err != nil {
					return err
				}
			} else if dividend.HoldingLotID != nil {
				if err := models.DeleteLotByID(tx, *dividend.HoldingLotID); err != nil {
					return err
				}
			}
			holding.TotalDividends = holding.TotalDividends.Sub(dividend.NetAmount)
			lots, err := models.LoadLots(tx, holding.ID)
			if err != nil {
				return err
			}
			models.RecalcFromLots(&holding, lots)
			if err := tx.Model(&holding).Updates(map[string]any{
				"shares": holding.Shares, "value": holding.Value, "cost": holding.Cost, "cost_price": holding.CostPrice,
				"total_dividends": holding.TotalDividends,
			}).Error; err != nil {
				return err
			}
			if err := tx.Where("id = ? AND user_id = ?", dividend.FundTxID, user.UserID).Delete(&models.FundTransaction{}).Error; err != nil {
				return err
			}
			return tx.Delete(&dividend).Error
		})
		if err != nil {
			writeDividendError(c, err)
			return
		}
		c.Status(consts.StatusNoContent)
	}
}
