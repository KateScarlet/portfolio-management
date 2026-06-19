package handlers

import (
	"context"
	"log/slog"
	"permanent-portfolio/yahoo"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func GetPrice() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		symbol := c.Param("symbol")
		if symbol == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "Symbol is required"})
			return
		}

		result, err := yahoo.FetchQuote(symbol)
		if err != nil {
			slog.Error("failed to fetch quote", "symbol", symbol, "error", err)
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusOK, result)
	}
}
