package handlers

import (
	"context"
	"encoding/json"
	"errors"
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

type CreateHoldingInput struct {
	models.Holding
	DeductFromCash bool `json:"deductFromCash"`
}

func CreateHolding(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var input CreateHoldingInput
		if err := c.BindJSON(&input); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		// Input validation
		if input.AssetId == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "assetId is required"})
			return
		}
		validAssets := map[string]bool{"stocks": true, "bonds": true, "cash": true, "gold": true}
		if !validAssets[input.AssetId] {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid assetId"})
			return
		}
		if input.Shares < 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "shares cannot be negative"})
			return
		}
		if input.Cost < 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "cost cannot be negative"})
			return
		}
		if input.CostPrice < 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "costPrice cannot be negative"})
			return
		}
		if input.Fee < 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "fee cannot be negative"})
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

		// Use a single transaction to atomically find-or-create, preventing
		// two concurrent requests from both creating a new holding for the same symbol/name.
		var created bool
		var result models.Holding
		err := db.Transaction(func(tx *gorm.DB) error {
			var existing models.Holding
			var res *gorm.DB
			if input.Symbol != "" {
				res = tx.Where("symbol = ? AND symbol != ''", input.Symbol).First(&existing)
			} else {
				res = tx.Where("name = ? AND asset_id = ? AND symbol = ''", input.Name, input.AssetId).First(&existing)
			}

			if res.Error == nil {
				// Found existing holding - merge into it
				existing.Shares += input.Shares

				switch {
				case existing.Cost > 0 && input.Cost > 0:
					existing.Cost += input.Cost
				case input.Cost > 0:
					existing.Cost = input.Cost
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
				created = false
				result = existing
				if err := tx.Save(&existing).Error; err != nil {
					return err
				}
			} else {
				if !errors.Is(res.Error, gorm.ErrRecordNotFound) {
					return res.Error
				}

				// No existing holding - create new
				input.ID = uuid.New().String()
				if input.Fee > 0 {
					input.Cost += input.Fee
				}
				if input.Lots == nil {
					input.Lots = models.JSONColumn{newLot}
				} else {
					input.Lots = append(input.Lots, newLot)
				}
				created = true
				result = input.Holding
				if err := tx.Create(&input.Holding).Error; err != nil {
					return err
				}
			}

			// Handle deduct from cash in the same transaction
			// Use newLot.Cost which holds the original cost (before fee was added to input.Cost)
			if input.DeductFromCash {
				addedCost := newLot.Cost + input.Fee
				if addedCost > 0 {
					var cashHolding models.Holding
					if err := tx.Where("asset_id = ?", "cash").First(&cashHolding).Error; err != nil {
						if errors.Is(err, gorm.ErrRecordNotFound) {
							// No cash holding found, skip deduction
							return nil
						}
						return err
					}

					cashHolding.Value -= addedCost
					if cashHolding.Value < 0 {
						cashHolding.Value = 0
					}

					cashHolding.Cost -= addedCost
					if cashHolding.Cost < 0 {
						cashHolding.Cost = 0
					}

					if err := tx.Save(&cashHolding).Error; err != nil {
						return err
					}
				}
			}

			return nil
		})
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if created {
			c.JSON(consts.StatusCreated, result)
		} else {
			c.JSON(consts.StatusOK, result)
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

		var updates map[string]any
		if err := c.BindJSON(&updates); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		// Whitelist of allowed fields to prevent mass assignment
		allowedFields := map[string]bool{
			"name": true, "symbol": true, "shares": true,
			"price": true, "costPrice": true, "value": true,
			"cost": true, "date": true, "lots": true,
		}
		safeUpdates := make(map[string]any)
		for k, v := range updates {
			if allowedFields[k] {
				safeUpdates[k] = v
			}
		}

		if lotsRaw, ok := safeUpdates["lots"]; ok {
			if lotsBytes, err := json.Marshal(lotsRaw); err == nil {
				var lots []models.HoldingLot
				if json.Unmarshal(lotsBytes, &lots) == nil {
					for i := range lots {
						if lots[i].ID == "" {
							lots[i].ID = uuid.New().String()
						}
					}
					safeUpdates["lots"] = models.JSONColumn(lots)
				}
			}
		}

		if len(safeUpdates) == 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "no valid fields to update"})
			return
		}

		if err := db.Model(&holding).Updates(safeUpdates).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if err := db.First(&holding, "id = ?", id).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
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
