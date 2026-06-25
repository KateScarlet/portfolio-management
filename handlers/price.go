package handlers

import (
	"context"
	"log/slog"
	"portfolio-management/eastmoney"
	"portfolio-management/yahoo"

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

		market := c.Query("market")

		var result any
		var err error
		switch market {
		case "FUND":
			result, err = eastmoney.FetchFundQuote(symbol)
		case "CN":
			result, err = eastmoney.FetchAShareQuote(symbol)
		default:
			if eastmoney.IsFuturesSymbol(symbol) {
				result, err = eastmoney.FetchQuote(symbol)
			} else {
				result, err = yahoo.FetchQuote(symbol)
			}
		}
		if err != nil {
			slog.Error("failed to fetch quote", "symbol", symbol, "market", market, "error", err)
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusOK, result)
	}
}
