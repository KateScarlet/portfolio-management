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

func ListPortfolios(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		var portfolios []models.Portfolio
		if err := db.Where("user_id = ?", user.UserID).Order("is_default DESC, created_at ASC").Find(&portfolios).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(consts.StatusOK, portfolios)
	}
}

func CreatePortfolio(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		var body struct {
			Name        string `json:"name" vd:"len($)>0"`
			Description string `json:"description"`
		}
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		portfolio := models.Portfolio{
			ID:          uuid.New().String(),
			UserID:      user.UserID,
			Name:        body.Name,
			Description: body.Description,
			IsDefault:   false,
			CreatedAt:   time.Now().UnixMilli(),
		}

		if err := db.Create(&portfolio).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusCreated, portfolio)
	}
}

func UpdatePortfolio(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		id := c.Param("id")
		var portfolio models.Portfolio
		if err := db.Where("user_id = ? AND id = ?", user.UserID, id).First(&portfolio).Error; err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "Portfolio not found"})
			return
		}

		var body struct {
			Name        *string `json:"name"`
			Description *string `json:"description"`
		}
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		updates := map[string]any{}
		if body.Name != nil {
			updates["name"] = *body.Name
		}
		if body.Description != nil {
			updates["description"] = *body.Description
		}

		if len(updates) == 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "no fields to update"})
			return
		}

		if err := db.Model(&portfolio).Updates(updates).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		db.Where("user_id = ? AND id = ?", user.UserID, id).First(&portfolio)
		c.JSON(consts.StatusOK, portfolio)
	}
}

func DeletePortfolio(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		id := c.Param("id")

		var portfolio models.Portfolio
		if err := db.Where("user_id = ? AND id = ?", user.UserID, id).First(&portfolio).Error; err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "Portfolio not found"})
			return
		}

		if portfolio.IsDefault {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "不能删除默认组合"})
			return
		}

		var count int64
		db.Model(&models.Portfolio{}).Where("user_id = ?", user.UserID).Count(&count)
		if count <= 1 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "至少需要保留一个投资组合"})
			return
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("portfolio_id = ?", id).Delete(&models.Holding{}).Error; err != nil {
				return err
			}
			if err := tx.Where("portfolio_id = ?", id).Delete(&models.PortfolioRecord{}).Error; err != nil {
				return err
			}
			if err := tx.Where("portfolio_id = ?", id).Delete(&models.Setting{}).Error; err != nil {
				return err
			}
			if err := tx.Delete(&portfolio).Error; err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusOK, map[string]bool{"success": true})
	}
}
