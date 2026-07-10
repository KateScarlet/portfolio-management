package handlers

import (
	"context"
	"log/slog"
	"portfolio-management/internal/marketsource"
	"portfolio-management/middleware"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
)

func GetPrice(router *marketsource.Router) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		symbol := c.Param("symbol")
		if symbol == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "股票代码不能为空"})
			return
		}

		market := c.Query("market")

		// Normalize symbol to canonical format
		if market != "" {
			symbol = marketsource.NormalizeSymbol(symbol, market)
		}

		// Try to get userID from auth context (may be empty for unauthenticated requests)
		var userID uuid.UUID
		if user := middleware.GetUser(c); user != nil {
			userID = user.UserID
		}

		result, err := router.FetchQuote(userID, symbol, market)
		if err != nil {
			slog.Error("failed to fetch quote", "symbol", symbol, "market", market, "error", err)
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusOK, result)
	}
}
