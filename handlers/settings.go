package handlers

import (
	"context"
	"portfolio-management/models"
	"portfolio-management/scheduler"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"gorm.io/gorm"
)

func ListSettings(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var settings []models.Setting
		if err := db.Find(&settings).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		result := make(map[string]string)
		for _, s := range settings {
			result[s.Key] = s.Value
		}
		c.JSON(consts.StatusOK, result)
	}
}

func UpdateSetting(db *gorm.DB, s *scheduler.PriceScheduler) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		key := c.Param("key")
		var body struct {
			Value string `json:"value"`
		}
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if body.Value == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "value is required"})
			return
		}

		// Validate before persisting
		if key == "syncInterval" {
			mins, err := strconv.Atoi(body.Value)
			if err != nil {
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "syncInterval must be a valid integer"})
				return
			}
			if mins < 0 || mins > 10080 {
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "syncInterval must be between 0 and 10080 minutes (7 days)"})
				return
			}
		}

		setting := models.Setting{Key: key, Value: body.Value}
		result := db.Where(models.Setting{Key: key}).Assign(models.Setting{Value: body.Value}).FirstOrCreate(&setting)
		if result.Error != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": result.Error.Error()})
			return
		}

		if key == "syncInterval" && s != nil {
			mins, _ := strconv.Atoi(body.Value)
			if mins == 0 {
				s.UpdateInterval(0)
			} else {
				s.UpdateInterval(time.Duration(mins) * time.Minute)
			}
		}

		c.JSON(consts.StatusOK, map[string]string{"key": key, "value": body.Value})
	}
}

func BatchUpdateSettings(db *gorm.DB, s *scheduler.PriceScheduler) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var body map[string]string
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if len(body) == 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "no settings provided"})
			return
		}

		for key, value := range body {
			if key == "syncInterval" || key == "driftThreshold" {
				if value == "" {
					c.JSON(consts.StatusBadRequest, map[string]string{"error": "value is required for key: " + key})
					return
				}
			}
		}

		// Validate syncInterval before persisting
		if syncVal, ok := body["syncInterval"]; ok {
			mins, err := strconv.Atoi(syncVal)
			if err != nil {
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "syncInterval must be a valid integer"})
				return
			}
			if mins < 0 || mins > 10080 {
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "syncInterval must be between 0 and 10080 minutes (7 days)"})
				return
			}
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			for key, value := range body {
				setting := models.Setting{Key: key, Value: value}
				if err := tx.Where(models.Setting{Key: key}).Assign(models.Setting{Value: value}).FirstOrCreate(&setting).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if s != nil {
			if syncVal, ok := body["syncInterval"]; ok {
				mins, _ := strconv.Atoi(syncVal)
				if mins == 0 {
					s.UpdateInterval(0)
				} else {
					s.UpdateInterval(time.Duration(mins) * time.Minute)
				}
			}
		}

		c.JSON(consts.StatusOK, body)
	}
}
