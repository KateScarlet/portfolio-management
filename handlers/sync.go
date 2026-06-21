package handlers

import (
	"context"
	"portfolio-management/scheduler"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func GetSyncStatus(s *scheduler.PriceScheduler) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, s.GetStatus())
	}
}

func TriggerSync(s *scheduler.PriceScheduler) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if !s.TryStartSync() {
			c.JSON(consts.StatusConflict, map[string]string{"error": "sync already in progress"})
			return
		}
		c.JSON(consts.StatusOK, s.GetStatus())
	}
}
