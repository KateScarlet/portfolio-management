package handlers

import (
	"context"
	"fmt"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"gorm.io/gorm"
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
		if !userOwnsPortfolio(db, user.UserID, portfolioID) {
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

		tx := db.Begin()
		if tx.Error != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": tx.Error.Error()})
			return
		}

		var holding models.Holding
		if err := tx.Where("portfolio_id = ?", portfolioID).First(&holding, "id = ?", id).Error; err != nil {
			tx.Rollback()
			c.JSON(consts.StatusNotFound, map[string]string{"error": "Holding not found"})
			return
		}

		var realizedValue float64
		var costReduction float64

		switch {
		case input.Shares > 0:
			// Standard sell: shares-based
			if input.Shares > holding.Shares {
				tx.Rollback()
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "Shares exceed holding"})
				return
			}
			if input.Price <= 0 {
				tx.Rollback()
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "Price must be greater than 0"})
				return
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
			// Manual holding sell: value-based (shares=0)
			if holding.Symbol != "" {
				tx.Rollback()
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "股票类持仓必须使用股数卖出，不能使用金额卖出"})
				return
			}
			if input.Value > holding.Value {
				tx.Rollback()
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "Value exceed holding"})
				return
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
			tx.Rollback()
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "Shares or value required"})
			return
		}

		// Validate fee doesn't exceed gross proceeds
		var grossProceeds float64
		if input.Shares > 0 {
			grossProceeds = input.Shares * input.Price
		} else {
			grossProceeds = input.Value
		}
		if input.Fee > 0 && input.Fee >= grossProceeds {
			tx.Rollback()
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "Fee cannot exceed sell proceeds"})
			return
		}

		// Create sell lot
		// ValueAdded = value removed from the holding (NOT the proceeds)
		// For share-based: shares * sellPrice (market value removed at actual sell price)
		// For value-based: input.Value (value removed)
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

		// Update holding with recalculated fields
		updates := map[string]any{
			"shares":    holding.Shares,
			"value":     holding.Value,
			"cost":      holding.Cost,
			"costPrice": holding.CostPrice,
			"lots":      holding.Lots,
		}
		if err := tx.Model(&holding).Updates(updates).Error; err != nil {
			tx.Rollback()
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Add proceeds to available funds (in holding's currency)
		var newFundsAmount float64
		if realizedValue > 0 {
			currency := holding.Currency
			if currency == "" {
				currency = "CNY"
			}

			var af models.AvailableFund
			err := tx.Where("user_id = ? AND portfolio_id = ? AND currency = ?", user.UserID, portfolioID, currency).First(&af).Error
			switch err {
			case nil:
				newFundsAmount = af.Amount + realizedValue
				if err := tx.Model(&af).Update("amount", newFundsAmount).Error; err != nil {
					tx.Rollback()
					c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
			case gorm.ErrRecordNotFound:
				newFundsAmount = realizedValue
				if err := tx.Create(&models.AvailableFund{
					ID:          uuid.New().String(),
					UserID:      user.UserID,
					PortfolioID: portfolioID,
					Currency:    currency,
					Amount:      newFundsAmount,
				}).Error; err != nil {
					tx.Rollback()
					c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
			default:
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
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
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}

		if err := tx.Commit().Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusOK, map[string]any{
			"soldHolding":    holding,
			"availableFunds": fmt.Sprintf("%.2f", newFundsAmount),
		})
	}
}
