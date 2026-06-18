package handlers

import (
	"context"
	"permanent-portfolio/yahoo"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func GetExchange() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		pair := c.Param("pair")
		if pair == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "Pair is required"})
			return
		}

		rate, err := yahoo.FetchExchangeRate(pair)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusOK, map[string]float64{"rate": rate})
	}
}
