package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"permanent-portfolio/db"
	"permanent-portfolio/handlers"
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
	database, err := db.Init("portfolio.db")
	if err != nil {
		panic("Failed to init database: " + err.Error())
	}

	yahoo.Init()

	// Read sync interval from settings, default 60 minutes
	var syncInterval time.Duration = 60 * time.Minute
	var setting models.Setting
	if database.Find(&setting, "key = ?", "syncInterval").Error == nil {
		if mins, err := strconv.Atoi(setting.Value); err == nil && mins > 0 {
			syncInterval = time.Duration(mins) * time.Minute
		} else if mins == 0 {
			syncInterval = 0 // disabled
		}
	}
	priceScheduler := scheduler.New(database, syncInterval)

	h := server.Default(server.WithHostPorts(":3000"))

	h.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:    []string{"Origin", "Content-Type", "Accept"},
	}))

	h.GET("/api/price/:symbol", handlers.GetPrice())
	h.GET("/api/exchange/:pair", handlers.GetExchange())
	h.GET("/api/sync/status", handlers.GetSyncStatus(priceScheduler))
	h.POST("/api/sync/trigger", handlers.TriggerSync(priceScheduler))

	api := h.Group("/api")
	{
		api.GET("/holdings", handlers.ListHoldings(database))
		api.POST("/holdings", handlers.CreateHolding(database))
		api.PATCH("/holdings/:id", handlers.UpdateHolding(database))
		api.DELETE("/holdings/:id", handlers.DeleteHolding(database))
		api.POST("/holdings/:id/sell", handlers.SellHolding(database))

		api.GET("/records", handlers.ListRecords(database))
		api.POST("/records", handlers.CreateRecord(database))
		api.DELETE("/records/:id", handlers.DeleteRecord(database))

		api.GET("/settings", handlers.ListSettings(database))
		api.PUT("/settings/:key", handlers.UpdateSetting(database, priceScheduler))
	}

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
