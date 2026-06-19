package handlers

import (
	"context"
	"encoding/json"
	"permanent-portfolio/models"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func ListHoldings(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var holdings []models.Holding
		if err := db.Order("asset_id").Find(&holdings).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(consts.StatusOK, holdings)
	}
}

func CreateHolding(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var input models.Holding
		if err := c.BindJSON(&input); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		var newLot models.HoldingLot
		newLot.ID = uuid.New().String()
		newLot.Date = input.Date
		if newLot.Date == 0 {
			newLot.Date = time.Now().UnixMilli()
		}
		newLot.Shares = input.Shares
		newLot.CostPrice = input.CostPrice
		newLot.Cost = input.Cost
		newLot.ValueAdded = input.Value
		newLot.Fee = input.Fee

		var existing models.Holding
		var result *gorm.DB
		if input.Symbol != "" {
			result = db.Where("symbol = ? AND symbol != ''", input.Symbol).First(&existing)
		} else {
			result = db.Where("name = ? AND asset_id = ? AND symbol = ''", input.Name, input.AssetId).First(&existing)
		}

		if result.Error == nil {
			existing.Shares += input.Shares

			switch {
			case existing.Cost > 0 && input.Cost > 0:
				existing.Cost += input.Cost
			case input.Cost > 0:
				existing.Cost = input.Cost
			case existing.Cost == 0 && input.Symbol == "":
				existing.Cost = input.Cost + input.Value
			}

			if input.Fee > 0 {
				existing.Cost += input.Fee
			}

			if existing.Shares > 0 && existing.Cost > 0 {
				existing.CostPrice = existing.Cost / existing.Shares
			}

			if existing.Symbol != "" && input.Price > 0 {
				existing.Price = input.Price
				existing.Value = existing.Shares * existing.Price
			} else {
				existing.Value += input.Value
			}

			existing.Lots = append(existing.Lots, newLot)

			if err := db.Save(&existing).Error; err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			c.JSON(consts.StatusOK, existing)
		} else {
			input.ID = uuid.New().String()
			if input.Fee > 0 {
				input.Cost += input.Fee
			}
			if input.Lots == nil {
				input.Lots = models.JSONColumn{newLot}
			} else {
				input.Lots = append(input.Lots, newLot)
			}
			if err := db.Create(&input).Error; err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			c.JSON(consts.StatusCreated, input)
		}
	}
}

func UpdateHolding(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		id := c.Param("id")
		var holding models.Holding
		if err := db.First(&holding, "id = ?", id).Error; err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "Holding not found"})
			return
		}

		var updates map[string]interface{}
		if err := c.BindJSON(&updates); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if lotsRaw, ok := updates["lots"]; ok {
			if lotsBytes, err := json.Marshal(lotsRaw); err == nil {
				var lots []models.HoldingLot
				if json.Unmarshal(lotsBytes, &lots) == nil {
					for i := range lots {
						if lots[i].ID == "" {
							lots[i].ID = uuid.New().String()
						}
					}
					updates["lots"] = models.JSONColumn(lots)
				}
			}
		}

		if err := db.Model(&holding).Updates(updates).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		db.First(&holding, "id = ?", id)
		c.JSON(consts.StatusOK, holding)
	}
}

func DeleteHolding(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		id := c.Param("id")
		result := db.Delete(&models.Holding{}, "id = ?", id)
		if result.Error != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": result.Error.Error()})
			return
		}
		if result.RowsAffected == 0 {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "Holding not found"})
			return
		}
		c.JSON(consts.StatusOK, map[string]bool{"success": true})
	}
}
