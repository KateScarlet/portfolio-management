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

		if !s.TriggerSyncForUser(user.UserID) {
			c.JSON(consts.StatusConflict, map[string]string{"error": "sync already in progress"})
			return
		}
		c.JSON(consts.StatusOK, s.GetStatusForUser(user.UserID))
	}
}
