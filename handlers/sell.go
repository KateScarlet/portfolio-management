package handlers

import (
	"context"
	"errors"
	"fmt"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"log/slog"
)

type SellRequest struct {
	Shares float64 `json:"shares"`
	Price  float64 `json:"price"`
	Value  float64 `json:"value"`
	Fee    float64 `json:"fee"`
}

func SellHolding(db *gorm.DB) app.HandlerFunc {
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

		var input SellRequest
		if err := c.BindJSON(&input); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if input.Fee < 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "Fee cannot be negative"})
			return
		}
		if input.Shares < 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "Shares cannot be negative"})
			return
		}
		if input.Value < 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "Value cannot be negative"})
			return
		}
		if input.Shares == 0 && input.Value == 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "Shares or value required"})
			return
		}

		var holding models.Holding
		var realizedValue float64
		var costReduction float64
		var newFundsAmount float64

		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("portfolio_id = ?", portfolioID).First(&holding, "id = ?", id).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return &httpError{status: consts.StatusNotFound, msg: "Holding not found"}
				}
				return err
			}

			switch {
			case input.Shares > 0:
				if input.Shares > holding.Shares {
					return &httpError{status: consts.StatusBadRequest, msg: "Shares exceed holding"}
				}
				if input.Price <= 0 {
					return &httpError{status: consts.StatusBadRequest, msg: "Price must be greater than 0"}
				}
				realizedValue = input.Shares*input.Price - input.Fee
				if holding.Shares > 0 {
					if input.Shares >= holding.Shares {
						costReduction = holding.Cost
					} else {
						costReduction = (holding.Cost / holding.Shares) * input.Shares
					}
				}
			case input.Value > 0:
				if holding.Symbol != "" {
					return &httpError{status: consts.StatusBadRequest, msg: "股票类持仓必须使用股数卖出，不能使用金额卖出"}
				}
				if input.Value > holding.Value {
					return &httpError{status: consts.StatusBadRequest, msg: "Value exceed holding"}
				}
				realizedValue = input.Value - input.Fee
				if holding.Value > 0 {
					if input.Value >= holding.Value {
						costReduction = holding.Cost
					} else {
						costReduction = (holding.Cost / holding.Value) * input.Value
					}
				} else if holding.Cost > 0 {
					costReduction = holding.Cost
				}
			default:
				return &httpError{status: consts.StatusBadRequest, msg: "Shares or value required"}
			}

			var grossProceeds float64
			if input.Shares > 0 {
				grossProceeds = input.Shares * input.Price
			} else {
				grossProceeds = input.Value
			}
			if input.Fee > 0 && input.Fee >= grossProceeds {
				return &httpError{status: consts.StatusBadRequest, msg: "Fee cannot exceed sell proceeds"}
			}

			sellLot := models.HoldingLot{
				ID:        uuid.New().String(),
				Type:      "sell",
				Date:      time.Now().UnixMilli(),
				CostPrice: holding.CostPrice,
				Cost:      costReduction,
				Fee:       input.Fee,
			}
			if input.Shares > 0 {
				sellLot.Shares = input.Shares
				sellLot.ValueAdded = input.Shares * input.Price
			} else {
				sellLot.ValueAdded = input.Value
			}

			holding.Lots = append(holding.Lots, sellLot)
			holding.RecalcFromLots()

			updates := map[string]any{
				"shares":    holding.Shares,
				"value":     holding.Value,
				"cost":      holding.Cost,
				"costPrice": holding.CostPrice,
				"lots":      holding.Lots,
			}
			if err := tx.Model(&holding).Updates(updates).Error; err != nil {
				return err
			}

			if realizedValue > 0 {
				currency := holding.Currency
				if currency == "" {
					currency = "CNY"
				}

				var af models.AvailableFund
				err := tx.Where("user_id = ? AND portfolio_id = ? AND currency = ?", user.UserID, portfolioID, currency).First(&af).Error
				switch {
				case err == nil:
					newFundsAmount = af.Amount + realizedValue
					if err := tx.Model(&af).Update("amount", newFundsAmount).Error; err != nil {
						return err
					}
				case errors.Is(err, gorm.ErrRecordNotFound):
					newFundsAmount = realizedValue
					if err := tx.Create(&models.AvailableFund{ID: uuid.New().String(), UserID: user.UserID, PortfolioID: portfolioID, Currency: currency, Amount: newFundsAmount}).Error; err != nil {
						return err
					}
				default:
					return err
				}

				if err := tx.Create(&models.FundTransaction{
					ID:          uuid.New().String(),
					UserID:      user.UserID,
					PortfolioID: portfolioID,
					Type:        "sell",
					Amount:      realizedValue,
					Currency:    currency,
					HoldingID:   holding.ID,
					CreatedAt:   time.Now().UnixMilli(),
				}).Error; err != nil {
					return err
				}
			}

			return nil
		})
		if err != nil {
			var he *httpError
			if errors.As(err, &he) {
				c.JSON(he.status, map[string]string{"error": he.msg})
			} else {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}

		c.JSON(consts.StatusOK, map[string]any{
			"soldHolding":    holding,
			"availableFunds": fmt.Sprintf("%.2f", newFundsAmount),
		})
	}
}
