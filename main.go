package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"permanent-portfolio/db"
	"permanent-portfolio/handlers"
	"permanent-portfolio/middleware"
	"permanent-portfolio/models"
	"permanent-portfolio/scheduler"
	"permanent-portfolio/yahoo"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/hertz-contrib/cors"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := db.LoadConfig()
	database, err := db.Init(cfg)
	if err != nil {
		slog.Error("failed to init database", "error", err)
		panic("Failed to init database: " + err.Error())
	}

	jwtSecret := db.GenerateJWTSecret()
	middleware.SetJWTSecret(jwtSecret)

	yahoo.Init()

	var syncInterval time.Duration = 60 * time.Minute
	var setting models.Setting
	if database.Find(&setting, "key = ?", "syncInterval").Error == nil {
		if mins, err := strconv.Atoi(setting.Value); err == nil && mins > 0 {
			syncInterval = time.Duration(mins) * time.Minute
		} else if mins == 0 {
			syncInterval = 0
		}
	}
	priceScheduler := scheduler.New(database, syncInterval)
	notifier := scheduler.NewNotifier(database)
	priceScheduler.SetNotifier(notifier)

	h := server.Default(server.WithHostPorts(":3000"))

	h.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		AllowCredentials: true,
	}))

	h.GET("/api/setup/status", handlers.SetupStatus())
	h.POST("/api/setup/complete", handlers.SetupComplete(database))

	h.POST("/api/auth/login", handlers.Login(database))
	h.POST("/api/auth/logout", handlers.Logout())
	h.GET("/api/auth/me", middleware.AuthRequired(), handlers.Me(database))

	h.GET("/api/price/:symbol", handlers.GetPrice())
	h.GET("/api/exchange/:pair", handlers.GetExchange())

	api := h.Group("/api")
	api.Use(middleware.AuthRequired())

	api.GET("/sync/status", handlers.GetSyncStatus(priceScheduler))
	api.POST("/sync/trigger", handlers.TriggerSync(priceScheduler))

	api.GET("/holdings", handlers.ListHoldings(database))
	api.POST("/holdings", handlers.CreateHolding(database))
	api.PATCH("/holdings/:id", handlers.UpdateHolding(database))
	api.DELETE("/holdings/:id", handlers.DeleteHolding(database))
	api.POST("/holdings/:id/sell", handlers.SellHolding(database))

	api.GET("/records", handlers.ListRecords(database))
	api.POST("/records", handlers.CreateRecord(database))
	api.DELETE("/records/:id", handlers.DeleteRecord(database))

	api.GET("/settings", handlers.ListSettings(database))
	api.PUT("/settings", handlers.BatchUpdateSettings(database, priceScheduler))
	api.PUT("/settings/:key", handlers.UpdateSetting(database, priceScheduler))
	api.POST("/telegram/test", handlers.TestTelegramMessage(database))

	admin := api.Group("")
	admin.Use(middleware.AdminRequired())
	admin.GET("/users", handlers.ListUsers(database))
	admin.POST("/users", handlers.Register(database))
	admin.DELETE("/users/:id", handlers.DeleteUser(database))

	distPath := filepath.Join(".", "web", "dist")
	if _, err := os.Stat(distPath); err == nil {
		fileServer := http.FileServer(http.Dir(distPath))
		h.GET("/*filepath", adaptor.HertzHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if path == "/" || path == "" {
				http.ServeFile(w, r, filepath.Join(distPath, "index.html"))
				return
			}
			fullPath := filepath.Join(distPath, path)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				if !strings.HasPrefix(path, "/api/") {
					http.ServeFile(w, r, filepath.Join(distPath, "index.html"))
					return
				}
				http.NotFound(w, r)
				return
			}
			fileServer.ServeHTTP(w, r)
		})))
	} else {
		h.GET("/*filepath", func(ctx context.Context, c *app.RequestContext) {
			c.String(consts.StatusNotFound, "Frontend not built. Run 'cd web && npm run build' first.")
		})
	}

	h.Spin()
}
