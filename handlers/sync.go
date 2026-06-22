package handlers

import (
	"context"
	"portfolio-management/middleware"
	"portfolio-management/scheduler"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func GetSyncStatus(s *scheduler.PriceScheduler) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		c.JSON(consts.StatusOK, s.GetStatusForUser(user.UserID))
	}
}

func TriggerSync(s *scheduler.PriceScheduler) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		status, ok := s.TriggerSyncForUserSync(user.UserID)
		if !ok {
			c.JSON(consts.StatusConflict, status)
			return
		}
		c.JSON(consts.StatusOK, status)
	}
}
