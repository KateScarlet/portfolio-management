package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"portfolio-management/models"
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
				existing.Lots = append(existing.Lots, newLot)

				// Update price if symbol-based and new price is provided
				if existing.Symbol != "" && input.Price > 0 {
					existing.Price = input.Price
				}

				// RecalcFromLots is the single source of truth for financial calculations
				existing.RecalcFromLots()

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
				if input.Lots == nil {
					input.Lots = models.JSONColumn{newLot}
				} else {
					input.Lots = append(input.Lots, newLot)
				}
				input.RecalcFromLots()
				created = true
				result = input.Holding
				if err := tx.Create(&input.Holding).Error; err != nil {
					return err
				}
			}

			// Handle deduct from available funds in the same transaction.
			if input.DeductFromCash {
				addedCost := newLot.Cost + input.Fee
				if addedCost > 0 {
					// Read current available funds
					var fundsSetting models.Setting
					fundsValue := 0.0
					if err := tx.Where("`key` = ?", "availableFunds").First(&fundsSetting).Error; err == nil {
						fmt.Sscanf(fundsSetting.Value, "%f", &fundsValue)
					}

					if fundsValue < addedCost {
						return fmt.Errorf("可用资金不足: 可用 %.2f, 需要 %.2f", fundsValue, addedCost)
					}

					newFundsValue := fundsValue - addedCost
					fundsSetting.Key = "availableFunds"
					fundsSetting.Value = fmt.Sprintf("%.2f", newFundsValue)
					if err := tx.Where("`key` = ?", "availableFunds").Assign(models.Setting{Value: fundsSetting.Value}).FirstOrCreate(&fundsSetting).Error; err != nil {
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

		// Whitelist of allowed fields to prevent mass assignment.
		// Note: assetId is intentionally excluded — changing it would break
		// portfolio composition integrity (e.g. reclassifying stocks as bonds).
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

		if _, ok := updates["assetId"]; ok {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "assetId cannot be changed"})
			return
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
					// Set lots and recalculate all derived fields
					holding.Lots = lots
					holding.RecalcFromLots()

					// Remove lots from safeUpdates; add recalculated fields
					delete(safeUpdates, "lots")
					safeUpdates["shares"] = holding.Shares
					safeUpdates["cost"] = holding.Cost
					safeUpdates["costPrice"] = holding.CostPrice

					// If price is also being updated, recompute value with the new price
					if newPrice, ok := safeUpdates["price"]; ok {
						if price, ok := newPrice.(float64); ok && holding.Symbol != "" {
							holding.Price = price
							holding.Value = holding.Shares * price
						}
					}
					safeUpdates["value"] = holding.Value
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
