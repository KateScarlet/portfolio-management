package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"portfolio-management/db"
	"strconv"
	"strings"
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
			DatabaseType     string `json:"databaseType"`
			DatabaseDSN      string `json:"databaseDsn"`
			DatabaseHost     string `json:"databaseHost"`
			DatabasePort     string `json:"databasePort"`
			DatabaseName     string `json:"databaseName"`
			DatabaseUsername string `json:"databaseUsername"`
			DatabasePassword string `json:"databasePassword"` //nolint:gosec // Request body field
			DatabaseSSLMode  string `json:"databaseSslMode"`
			Username         string `json:"username"`
			Password         string `json:"password"` //nolint:gosec // Request body field
		}
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if body.DatabaseType == "" {
			body.DatabaseType = "postgres"
		}
		if body.DatabaseDSN == "" {
			var err error
			body.DatabaseDSN, err = buildPostgresDSN(
				body.DatabaseHost,
				body.DatabasePort,
				body.DatabaseName,
				body.DatabaseUsername,
				body.DatabasePassword,
				body.DatabaseSSLMode,
			)
			if err != nil {
				c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
				return
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
			if err := h.Shutdown(context.Background()); err != nil {
				slog.Error("failed to shutdown server", "error", err)
			}
		}()
	}
}

func buildPostgresDSN(host, port, name, username, password, sslMode string) (string, error) {
	host = strings.TrimSpace(host)
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	port = strings.TrimSpace(port)
	name = strings.TrimSpace(name)
	username = strings.TrimSpace(username)
	sslMode = strings.TrimSpace(sslMode)

	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return "", errors.New("数据库端口必须是 1 到 65535 之间的数字")
	}
	if name == "" {
		name = "portfolio"
	}
	if sslMode == "" {
		sslMode = "disable"
	}
	validSSLModes := map[string]bool{
		"disable": true, "allow": true, "prefer": true, "require": true,
		"verify-ca": true, "verify-full": true,
	}
	if !validSSLModes[sslMode] {
		return "", fmt.Errorf("不支持的 SSL 模式: %s", sslMode)
	}

	dsn := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(host, port),
		Path:   name,
	}
	if username != "" {
		if password == "" {
			dsn.User = url.User(username)
		} else {
			dsn.User = url.UserPassword(username, password)
		}
	}
	query := url.Values{}
	query.Set("sslmode", sslMode)
	dsn.RawQuery = query.Encode()
	return dsn.String(), nil
}
