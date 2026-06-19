package handlers

import (
	"context"
	"permanent-portfolio/scheduler"

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
		go s.SyncNow()
		c.JSON(consts.StatusOK, s.GetStatus())
	}
}
