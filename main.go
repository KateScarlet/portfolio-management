package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"portfolio-management/db"
	"portfolio-management/handlers"
	"portfolio-management/middleware"
	"portfolio-management/scheduler"
	"portfolio-management/yahoo"
	"strings"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
	"github.com/hertz-contrib/cors"
)

//go:embed web/dist/*
var frontendFS embed.FS

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := db.LoadConfig()
	middleware.SetJWTSecret(cfg.JWTSecret)

	h := server.Default(server.WithHostPorts(":3000"))

	h.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept"},
		AllowCredentials: true,
	}))

	h.GET("/api/setup/status", handlers.SetupStatus())
	h.POST("/api/setup/complete", handlers.SetupComplete(h))

	if db.IsSetupMode() {
		serveFrontend(h)
		h.Spin()
		return
	}

	database, err := db.Init(cfg)
	if err != nil {
		slog.Error("failed to init database", "error", err)
		panic("Failed to init database: " + err.Error())
	}
	sqlDB, err := database.DB()
	if err != nil {
		slog.Error("failed to get sqlDB", "error", err)
		panic("Failed to get sqlDB: " + err.Error())
	}
	defer sqlDB.Close()

	yahoo.Init()

	priceScheduler := scheduler.New(database)
	notifier := scheduler.NewNotifier(database)
	priceScheduler.SetNotifier(notifier)

	h.POST("/api/auth/login", handlers.Login(database))
	h.POST("/api/auth/logout", handlers.Logout())
	h.GET("/api/auth/me", middleware.AuthRequired(), handlers.Me(database))

	h.GET("/api/auth/oidc", handlers.OIDCLogin(cfg))
	h.GET("/api/auth/oidc/callback", handlers.OIDCCallback(database, cfg))
	h.GET("/api/auth/oidc/status", handlers.OIDCStatus(cfg))

	h.GET("/api/webauthn/status", handlers.WebAuthnStatus(cfg))
	h.POST("/api/webauthn/login/start", handlers.WebAuthnLoginStart(database, cfg))
	h.POST("/api/webauthn/login/finish", handlers.WebAuthnLoginFinish(database, cfg))

	oidcAdmin := h.Group("/api/oidc")
	oidcAdmin.Use(middleware.AuthRequired(), middleware.AdminRequired())
	oidcAdmin.GET("/config", handlers.GetOIDCConfig(cfg))
	oidcAdmin.PUT("/config", handlers.UpdateOIDCConfig(cfg))
	oidcAdmin.GET("/webauthn-config", handlers.GetWebAuthnConfig(cfg))
	oidcAdmin.PUT("/webauthn-config", handlers.UpdateWebAuthnConfig(cfg))

	h.GET("/api/price/:symbol", handlers.GetPrice())
	h.GET("/api/exchange/:pair", handlers.GetExchange())

	api := h.Group("/api")
	api.Use(middleware.AuthRequired())

	api.GET("/portfolios", handlers.ListPortfolios(database))
	api.POST("/portfolios", handlers.CreatePortfolio(database))
	api.PATCH("/portfolios/:id", handlers.UpdatePortfolio(database))
	api.DELETE("/portfolios/:id", handlers.DeletePortfolio(database))

	api.GET("/summary", handlers.GetSummary(database))

	api.POST("/telegram/test", handlers.TestTelegramMessage(database))

	api.POST("/webauthn/register/start", handlers.WebAuthnRegisterStart(database, cfg))
	api.POST("/webauthn/register/finish", handlers.WebAuthnRegisterFinish(database, cfg))
	api.GET("/webauthn/credentials", handlers.WebAuthnListCredentials(database))
	api.DELETE("/webauthn/credentials/:id", handlers.WebAuthnDeleteCredential(database))

	pf := api.Group("/portfolios/:pid")
	pf.GET("/sync/status", handlers.GetSyncStatus(priceScheduler))
	pf.POST("/sync/trigger", handlers.TriggerSync(priceScheduler))

	pf.GET("/holdings", handlers.ListHoldings(database))
	pf.POST("/holdings", handlers.CreateHolding(database))
	pf.PATCH("/holdings/:id", handlers.UpdateHolding(database))
	pf.DELETE("/holdings/:id", handlers.DeleteHolding(database))
	pf.POST("/holdings/:id/sell", handlers.SellHolding(database))

	pf.GET("/records", handlers.ListRecords(database))
	pf.POST("/records", handlers.CreateRecord(database))
	pf.DELETE("/records/:id", handlers.DeleteRecord(database))

	pf.GET("/settings", handlers.ListSettings(database))
	pf.PUT("/settings", handlers.BatchUpdateSettings(database, priceScheduler))
	pf.PUT("/settings/:key", handlers.UpdateSetting(database, priceScheduler))
	pf.GET("/funds", handlers.GetAvailableFunds(database))
	pf.PUT("/funds", handlers.UpdateAvailableFunds(database))

	admin := api.Group("")
	admin.Use(middleware.AdminRequired())
	admin.GET("/users", handlers.ListUsers(database))
	admin.POST("/users", handlers.Register(database))
	admin.DELETE("/users/:id", handlers.DeleteUser(database))

	serveFrontend(h)

	h.Spin()
}

func serveFrontend(h *server.Hertz) {
	subFS, err := fs.Sub(frontendFS, "web/dist")
	if err != nil {
		slog.Error("failed to create sub filesystem", "error", err)
		panic("Failed to create sub filesystem: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(subFS))
	h.GET("/*filepath", adaptor.HertzHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" || path == "" {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFileFS(w, r, subFS, "index.html")
			return
		}
		if strings.HasPrefix(path, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		if _, err := fs.Stat(subFS, strings.TrimPrefix(path, "/")); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		if !strings.HasPrefix(path, "/api/") {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFileFS(w, r, subFS, "index.html")
			return
		}
		http.NotFound(w, r)
	})))
}
