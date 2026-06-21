package handlers

import (
	"context"
	"fmt"
	"permanent-portfolio/models"
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
		if err := tx.First(&holding, "id = ?", id).Error; err != nil {
			tx.Rollback()
			c.JSON(consts.StatusNotFound, map[string]string{"error": "Holding not found"})
			return
		}

		var realizedValue float64
		var remainingShares float64
		var remainingValue float64
		var costReduction float64

		switch {
		case input.Shares > 0:
			// Standard sell: shares-based
			if input.Shares > holding.Shares {
				tx.Rollback()
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "Shares exceed holding"})
				return
			}
			if input.Price < 0 {
				tx.Rollback()
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "Invalid price"})
				return
			}
			realizedValue = input.Shares*input.Price - input.Fee
			remainingShares = holding.Shares - input.Shares
			remainingValue = holding.Value
			if holding.Shares > 0 {
				costReduction = (holding.Cost / holding.Shares) * input.Shares
			}
		case input.Value > 0:
			// Manual holding sell: value-based (shares=0)
			if input.Value > holding.Value {
				tx.Rollback()
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "Value exceed holding"})
				return
			}
			realizedValue = input.Value - input.Fee
			remainingShares = 0
			remainingValue = holding.Value - input.Value
			if holding.Value > 0 {
				costReduction = (holding.Cost / holding.Value) * input.Value
			}
		default:
			tx.Rollback()
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "Shares or value required"})
			return
		}

		sellLot := models.HoldingLot{
			ID:         uuid.New().String(),
			Type:       "sell",
			Date:       time.Now().UnixMilli(),
			Shares:     input.Shares,
			CostPrice:  holding.CostPrice,
			Cost:       costReduction,
			ValueAdded: realizedValue + input.Fee,
			Fee:        input.Fee,
		}
		holding.Lots = append(holding.Lots, sellLot)

		switch {
		case remainingShares == 0 && remainingValue == 0:
			updates := map[string]any{
				"shares": 0,
				"value":  0,
				"cost":   0,
				"lots":   holding.Lots,
			}
			if err := tx.Model(&holding).Updates(updates).Error; err != nil {
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		case input.Shares > 0:
			remainingCost := holding.Cost
			if holding.Shares > 0 {
				remainingCost = (holding.Cost / holding.Shares) * remainingShares
			}
			updates := map[string]any{
				"shares": remainingShares,
				"value":  remainingShares * holding.Price,
				"cost":   remainingCost,
				"lots":   holding.Lots,
			}
			if err := tx.Model(&holding).Updates(updates).Error; err != nil {
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		default:
			remainingCost := holding.Cost
			if holding.Value > 0 {
				remainingCost = (holding.Cost / holding.Value) * remainingValue
			}
			updates := map[string]any{
				"value": remainingValue,
				"cost":  remainingCost,
				"lots":  holding.Lots,
			}
			if err := tx.Model(&holding).Updates(updates).Error; err != nil {
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}

		var cashHolding models.Holding
		if err := tx.Where("asset_id = ?", "cash").First(&cashHolding).Error; err == nil {
			cashHolding.Value += realizedValue
			cashHolding.Cost += realizedValue
			if err := tx.Save(&cashHolding).Error; err != nil {
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		} else {
			cashHolding = models.Holding{
				ID:      uuid.New().String(),
				AssetId: "cash",
				Name:    "可用现金",
				Value:   realizedValue,
				Cost:    realizedValue,
				Lots: models.JSONColumn{{
					ID:         uuid.New().String(),
					Date:       time.Now().UnixMilli(),
					Shares:     0,
					ValueAdded: realizedValue,
				}},
			}
			if err := tx.Create(&cashHolding).Error; err != nil {
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}

		if err := tx.Commit().Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Re-read both from DB after commit to ensure consistency.
		var holdings []models.Holding
		if err := db.Order("asset_id").Find(&holdings).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		var committedCash models.Holding
		if err := db.Where("asset_id = ?", "cash").First(&committedCash).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to read cash: %v", err)})
			return
		}

		c.JSON(consts.StatusOK, map[string]any{
			"holdings":    holdings,
			"cashHolding": committedCash,
		})
	}
}
