package handlers

import (
	"context"
	"log/slog"
	"portfolio-management/marketsource"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func GetExchange(router *marketsource.Router) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		pair := c.Param("pair")
		if pair == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "Pair is required"})
			return
		}

		rate, err := router.ExchangeRate("", pair)
		if err != nil {
			slog.Error("failed to fetch exchange rate", "pair", pair, "error", err)
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusOK, map[string]float64{"rate": rate})
	}
}
