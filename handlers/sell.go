package handlers

import (
	"context"
	"permanent-portfolio/models"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SellRequest struct {
	Shares float64 `json:"shares"`
	Price  float64 `json:"price"`
}

func SellHolding(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		id := c.Param("id")

		var input SellRequest
		if err := c.BindJSON(&input); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if input.Shares <= 0 || input.Price < 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "Invalid shares or price"})
			return
		}

		var holding models.Holding
		if err := db.First(&holding, "id = ?", id).Error; err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "Holding not found"})
			return
		}

		if input.Shares > holding.Shares {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "Shares exceed holding"})
			return
		}

		realizedValue := input.Shares * input.Price
		remainingShares := holding.Shares - input.Shares

		tx := db.Begin()

		if remainingShares == 0 {
			if err := tx.Delete(&holding).Error; err != nil {
				tx.Rollback()
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		} else {
			remainingCost := holding.Cost
			if holding.Shares > 0 {
				remainingCost = (holding.Cost / holding.Shares) * remainingShares
			}
			updates := map[string]interface{}{
				"shares": remainingShares,
				"value":  remainingShares * holding.Price,
				"cost":   remainingCost,
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
				Name:    "卖出资金 (现金)",
				Value:   realizedValue,
				Cost:    realizedValue,
				Lots: models.JSONColumn{{
					ID:         uuid.New().String(),
					Date:       holding.Date,
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

		tx.Commit()

		db.First(&holding, "id = ?", id)
		db.Where("asset_id = ?", "cash").First(&cashHolding)

		c.JSON(consts.StatusOK, map[string]interface{}{
			"holding":     holding,
			"cashHolding": cashHolding,
		})
	}
}
