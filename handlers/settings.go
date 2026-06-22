package handlers

import (
	"context"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"portfolio-management/scheduler"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"gorm.io/gorm"
)

func ListSettings(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		var settings []models.Setting
		if err := db.Where("user_id = ?", user.UserID).Find(&settings).Error; err != nil {
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
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

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

		setting := models.Setting{Key: key, Value: body.Value, UserID: user.UserID}
		result := db.Where(models.Setting{Key: key, UserID: user.UserID}).Assign(models.Setting{Value: body.Value}).FirstOrCreate(&setting)
		if result.Error != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": result.Error.Error()})
			return
		}

		c.JSON(consts.StatusOK, map[string]string{"key": key, "value": body.Value})
	}
}

func GetAvailableFunds(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		var setting models.Setting
		if err := db.Where("`key` = ? AND user_id = ?", "availableFunds", user.UserID).First(&setting).Error; err != nil {
			c.JSON(consts.StatusOK, map[string]string{"value": "0"})
			return
		}
		c.JSON(consts.StatusOK, map[string]string{"value": setting.Value})
	}
}

func UpdateAvailableFunds(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		var body struct {
			Value string `json:"value"`
		}
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		setting := models.Setting{Key: "availableFunds", Value: body.Value, UserID: user.UserID}
		result := db.Where(models.Setting{Key: "availableFunds", UserID: user.UserID}).Assign(models.Setting{Value: body.Value}).FirstOrCreate(&setting)
		if result.Error != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": result.Error.Error()})
			return
		}

		c.JSON(consts.StatusOK, map[string]string{"key": "availableFunds", "value": body.Value})
	}
}

func BatchUpdateSettings(db *gorm.DB, s *scheduler.PriceScheduler) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

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
				setting := models.Setting{Key: key, Value: value, UserID: user.UserID}
				if err := tx.Where(models.Setting{Key: key, UserID: user.UserID}).Assign(models.Setting{Value: value}).FirstOrCreate(&setting).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusOK, body)
	}
}
