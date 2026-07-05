package handlers

import (
	"context"
	"portfolio-management/middleware"
	"portfolio-management/scheduler"

	"log/slog"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TriggerSync(db *gorm.DB, s *scheduler.PriceScheduler) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID, err := uuid.Parse(c.Param("pid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的组合ID"})
			return
		}
		owns, err := userOwnsPortfolio(db, user.UserID, portfolioID)
		if err != nil {
			slog.Error("failed to check portfolio ownership", "error", err)
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "数据库错误"})
			return
		}
		if !owns {
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
