package handlers

import (
	"context"
	"log/slog"
	"portfolio-management/marketsource"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func GetPrice(router *marketsource.Router) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		symbol := c.Param("symbol")
		if symbol == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "Symbol is required"})
			return
		}

		market := c.Query("market")

		result, err := router.FetchQuote("", symbol, market)
		if err != nil {
			slog.Error("failed to fetch quote", "symbol", symbol, "market", market, "error", err)
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusOK, result)
	}
}
