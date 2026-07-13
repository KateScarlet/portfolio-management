package handlers

import (
	"context"
	"errors"
	"os"
	"portfolio-management/db"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"gorm.io/gorm"
)

var setupMu sync.Mutex

func SetupStatus() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, map[string]bool{
			"configured": !db.IsSetupMode(),
		})
	}
}

func SetupComplete(h *server.Hertz) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		setupMu.Lock()
		defer setupMu.Unlock()

		// The route can receive concurrent requests before the successful setup
		// shuts the server down. Only the first request may initialize the app.
		if !db.IsSetupMode() {
			c.JSON(consts.StatusConflict, map[string]string{"error": "系统已完成初始化"})
			return
		}

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
			body.DatabaseType = "postgres"
		}
		if body.DatabaseDSN == "" {
			body.DatabaseDSN = "postgres://localhost:5432/portfolio?sslmode=disable"
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

		database, err := db.Init(cfg)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "初始化数据库失败: " + err.Error()})
			return
		}

		// Keep administrator creation uncommitted until the config file has been
		// written. If either operation or the transaction commit fails, removing
		// the config keeps the application in setup mode and the DB rolls back.
		err = database.Transaction(func(tx *gorm.DB) error {
			if err := CreateUserForSetup(tx, body.Username, body.Password, "admin"); err != nil {
				return err
			}
			return db.SaveConfig(cfg)
		})
		if err != nil {
			cleanupErr := os.Remove(db.ConfigFile())
			if cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
				c.JSON(consts.StatusInternalServerError, map[string]string{
					"error": "初始化失败，且无法清理未完成的配置: " + cleanupErr.Error(),
				})
				return
			}
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "完成初始化失败: " + err.Error()})
			return
		}

		c.JSON(consts.StatusOK, map[string]bool{"success": true})

		go func() {
			time.Sleep(100 * time.Millisecond)
			h.Shutdown(context.Background())
		}()
	}
}
