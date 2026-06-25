package handlers

import (
	"context"
	"portfolio-management/db"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func SetupStatus() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, map[string]bool{
			"configured": !db.IsSetupMode(),
		})
	}
}

func SetupComplete(h *server.Hertz) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var body struct {
			DatabaseType string `json:"databaseType"`
			DatabaseDSN  string `json:"databaseDsn"`
			Username     string `json:"username"`
			Password     string `json:"password"` //nolint:gosec // Request body field
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

		if body.Username == "" || body.Password == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "管理员用户名和密码不能为空"})
			return
		}

		if len(body.Password) < 6 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "密码至少6位"})
			return
		}

		cfg := &db.Config{}
		cfg.Database.Type = body.DatabaseType
		cfg.Database.DSN = body.DatabaseDSN

		if err := db.SaveConfig(cfg); err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		database, err := db.Init(cfg)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "初始化数据库失败: " + err.Error()})
			return
		}

		if err := CreateUserForSetup(database, body.Username, body.Password, "admin"); err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "创建管理员失败: " + err.Error()})
			return
		}

		c.JSON(consts.StatusOK, map[string]bool{"success": true})

		go func() {
			time.Sleep(100 * time.Millisecond)
			h.Shutdown(ctx)
		}()
	}
}
