package handlers

import (
	"context"
	"permanent-portfolio/db"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func SetupStatus() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, map[string]bool{
			"configured": !db.IsSetupMode(),
		})
	}
}

func SetupComplete() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var body struct {
			DatabaseType string `json:"databaseType"`
			DatabaseDSN  string `json:"databaseDsn"`
		}
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if body.DatabaseType == "" {
			body.DatabaseType = "sqlite"
		}
		if body.DatabaseDSN == "" {
			if body.DatabaseType == "postgres" {
				body.DatabaseDSN = "postgres://localhost:5432/portfolio?sslmode=disable"
			} else {
				body.DatabaseDSN = "portfolio.db"
			}
		}

		cfg := &db.Config{}
		cfg.Database.Type = body.DatabaseType
		cfg.Database.DSN = body.DatabaseDSN

		if err := db.SaveConfig(cfg); err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusOK, map[string]bool{"success": true})
	}
}