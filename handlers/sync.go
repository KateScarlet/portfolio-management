package handlers

import (
	"context"
	"portfolio-management/middleware"
	"portfolio-management/scheduler"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"gorm.io/gorm"
)

func GetSyncStatus(db *gorm.DB, s *scheduler.PriceScheduler) app.HandlerFunc {
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

		c.JSON(consts.StatusOK, s.GetStatusForPortfolio(user.UserID, portfolioID))
	}
}

func TriggerSync(db *gorm.DB, s *scheduler.PriceScheduler) app.HandlerFunc {
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

		status, ok := s.TriggerSyncForPortfolioSync(user.UserID, portfolioID)
		if !ok {
			c.JSON(consts.StatusConflict, status)
			return
		}
		c.JSON(consts.StatusOK, status)
	}
}
