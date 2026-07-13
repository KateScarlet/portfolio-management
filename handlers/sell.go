package handlers

import (
	"context"
	"errors"
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

type SellRequest struct {
	Shares decimal.Decimal `json:"shares"`
	Price  decimal.Decimal `json:"price"`
	Value  decimal.Decimal `json:"value"`
	Fee    decimal.Decimal `json:"fee"`
	Date   *time.Time      `json:"date,omitempty"`
}

func SellHolding(db *gorm.DB) app.HandlerFunc {
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
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的持仓ID"})
			return
		}

		var input SellRequest
		if err := c.BindJSON(&input); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if input.Fee.IsNegative() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "手续费不能为负数"})
			return
		}
		if input.Shares.IsNegative() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "股数不能为负数"})
			return
		}
		if input.Value.IsNegative() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "金额不能为负数"})
			return
		}
		if input.Shares.IsZero() && input.Value.IsZero() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "需要提供股数或金额"})
			return
		}

		var holding models.Holding
		var realizedValue decimal.Decimal
		var costReduction decimal.Decimal
		var newFundsAmount decimal.Decimal

		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("portfolio_id = ?", portfolioID).First(&holding, "id = ?", id).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return &httpError{status: consts.StatusNotFound, msg: "持仓不存在"}
				}
				return err
			}

			lots, err := models.LoadLots(tx, holding.ID)
			if err != nil {
				return err
			}

			switch {
			case input.Shares.IsPositive():
				if input.Shares.GreaterThan(holding.Shares) {
					return &httpError{status: consts.StatusBadRequest, msg: "卖出股数超过持仓"}
				}
				if !input.Price.IsPositive() {
					return &httpError{status: consts.StatusBadRequest, msg: "价格必须大于0"}
				}
				realizedValue = input.Shares.Mul(input.Price).Sub(input.Fee)
				if holding.Shares.IsPositive() {
					if input.Shares.GreaterThanOrEqual(holding.Shares) {
						costReduction = holding.Cost
					} else {
						costReduction = holding.Cost.Div(holding.Shares).Mul(input.Shares)
					}
				}
			case input.Value.IsPositive():
				if holding.Symbol != "" {
					return &httpError{status: consts.StatusBadRequest, msg: "股票类持仓必须使用股数卖出，不能使用金额卖出"}
				}
				if input.Value.GreaterThan(holding.Value) {
					return &httpError{status: consts.StatusBadRequest, msg: "卖出金额超过持仓"}
				}
				realizedValue = input.Value.Sub(input.Fee)
				if holding.Value.IsPositive() {
					if input.Value.GreaterThanOrEqual(holding.Value) {
						costReduction = holding.Cost
					} else {
						costReduction = holding.Cost.Div(holding.Value).Mul(input.Value)
					}
				} else if holding.Cost.IsPositive() {
					costReduction = holding.Cost
				}
			default:
				return &httpError{status: consts.StatusBadRequest, msg: "需要提供股数或金额"}
			}

			var grossProceeds decimal.Decimal
			if input.Shares.IsPositive() {
				grossProceeds = input.Shares.Mul(input.Price)
			} else {
				grossProceeds = input.Value
			}
			if input.Fee.IsPositive() && !input.Fee.LessThan(grossProceeds) {
				return &httpError{status: consts.StatusBadRequest, msg: "手续费不能超过卖出收入"}
			}

			sellDate := time.Now()
			if input.Date != nil && !input.Date.IsZero() {
				sellDate = *input.Date
			}
			var sellPrice decimal.Decimal
			if input.Shares.IsPositive() {
				sellPrice = input.Price
			}

			sellLot := models.HoldingLot{
				ID:        uuid.New(),
				HoldingID: holding.ID,
				Type:      "sell",
				Date:      sellDate,
				CostPrice: sellPrice,
				Cost:      costReduction,
				Fee:       input.Fee,
			}
			if input.Shares.IsPositive() {
				sellLot.Shares = input.Shares
				sellLot.ValueAdded = input.Shares.Mul(input.Price)
			} else {
				sellLot.ValueAdded = input.Value
			}

			lots = append(lots, sellLot)
			models.RecalcFromLots(&holding, lots)

			updates := map[string]any{
				"shares":    holding.Shares,
				"value":     holding.Value,
				"cost":      holding.Cost,
				"costPrice": holding.CostPrice,
			}
			if err := tx.Model(&holding).Updates(updates).Error; err != nil {
				return err
			}
			if err := models.CreateLot(tx, &sellLot); err != nil {
				return err
			}

			if realizedValue.IsPositive() {
				currency := holding.Currency
				if currency == "" {
					currency = "CNY"
				}

				var af models.AvailableFund
				err := tx.Where("user_id = ? AND portfolio_id = ? AND currency = ?", user.UserID, portfolioID, currency).First(&af).Error
				switch {
				case err == nil:
					newFundsAmount = af.Amount.Add(realizedValue)
					if err := tx.Model(&af).Update("amount", newFundsAmount).Error; err != nil {
						return err
					}
				case errors.Is(err, gorm.ErrRecordNotFound):
					newFundsAmount = realizedValue
					if err := tx.Create(&models.AvailableFund{ID: uuid.New(), UserID: user.UserID, PortfolioID: portfolioID, Currency: currency, Amount: newFundsAmount}).Error; err != nil {
						return err
					}
				default:
					return err
				}

				if err := tx.Create(&models.FundTransaction{
					ID:          uuid.New(),
					UserID:      user.UserID,
					PortfolioID: portfolioID,
					Type:        "sell",
					Amount:      realizedValue,
					Currency:    currency,
					HoldingID:   &holding.ID,
				}).Error; err != nil {
					return err
				}
			}

			return nil
		})
		if err != nil {
			if he, ok := errors.AsType[*httpError](err); ok {
				c.JSON(he.status, map[string]string{"error": he.msg})
			} else {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}

		// Reload lots for response
		lots, err := models.LoadLots(db, holding.ID)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(consts.StatusOK, map[string]any{
			"soldHolding":    HoldingResponse{Holding: holding, Lots: lots},
			"availableFunds": newFundsAmount.StringFixed(2),
		})
	}
}
