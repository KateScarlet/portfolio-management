package handlers

import (
	"context"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func ListRecords(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		var records []models.PortfolioRecord
		if err := db.Where("user_id = ?", user.UserID).Order("timestamp DESC").Find(&records).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(consts.StatusOK, records)
	}
}

func CreateRecord(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		var holdings []models.Holding
		if err := db.Where("user_id = ?", user.UserID).Find(&holdings).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		assets := models.AssetMapColumn{"stocks": 0, "bonds": 0, "cash": 0, "gold": 0}
		var total, principal float64
		snapshotHoldings := make(models.HoldingSnapshotColumn, 0, len(holdings))
		for i := range holdings {
			assets[holdings[i].AssetId] += holdings[i].Value
			total += holdings[i].Value
			principal += holdings[i].Cost + holdings[i].BuyFees()

			if holdings[i].Value > 0 {
				snapshotHoldings = append(snapshotHoldings, models.HoldingSnapshot{
					AssetId:   holdings[i].AssetId,
					Symbol:    holdings[i].Symbol,
					Name:      holdings[i].Name,
					Shares:    holdings[i].Shares,
					Price:     holdings[i].Price,
					CostPrice: holdings[i].CostPrice,
					Value:     holdings[i].Value,
					Cost:      holdings[i].Cost,
				})
			}
		}

		if total == 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "No data to record"})
			return
		}

		record := models.PortfolioRecord{
			ID:        uuid.New().String(),
			UserID:    user.UserID,
			Timestamp: time.Now().UnixMilli(),
			Assets:    assets,
			Holdings:  snapshotHoldings,
			Total:     total,
			Principal: principal,
		}

		if err := db.Create(&record).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusCreated, record)
	}
}

func DeleteRecord(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		id := c.Param("id")
		result := db.Where("user_id = ?", user.UserID).Delete(&models.PortfolioRecord{}, "id = ?", id)
		if result.Error != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": result.Error.Error()})
			return
		}
		if result.RowsAffected == 0 {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "Record not found"})
			return
		}
		c.JSON(consts.StatusOK, map[string]bool{"success": true})
	}
}
