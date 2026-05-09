package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jcrlabs/compra-back/internal/config"
	"github.com/jcrlabs/compra-back/internal/database"
	"github.com/jcrlabs/compra-back/internal/handlers"
	"github.com/jcrlabs/compra-back/internal/middleware"
	"github.com/jcrlabs/compra-back/internal/repository"
	"github.com/jcrlabs/compra-back/internal/scraper"
	"github.com/jcrlabs/compra-back/internal/services"
)

const version = "0.1.0"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	db, err := database.Connect(cfg)
	if err != nil {
		slog.Error("db connect failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := database.Migrate(db); err != nil {
		slog.Error("migration failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Repositories
	userRepo := repository.NewUserRepository(db)
	catalogRepo := repository.NewCatalogRepository(db)
	pricingRepo := repository.NewPricingRepository(db)
	listRepo := repository.NewListRepository(db)

	// Services
	authSvc := services.NewAuthService(userRepo, cfg.JWTSecret, cfg.JWTAccessTTLHours)

	// Handlers
	authHandler := handlers.NewAuthHandler(authSvc)
	catalogHandler := handlers.NewCatalogHandler(catalogRepo)
	pricingHandler := handlers.NewPricingHandler(pricingRepo, userRepo)
	listHandler := handlers.NewListHandler(listRepo, pricingRepo, userRepo)
	healthHandler := handlers.NewHealthHandler(db, version)

	// Scrapers + scheduler
	ingester := scraper.NewIngester(catalogRepo, pricingRepo)
	scheduler := scraper.NewScheduler(ingester,
		scraper.NewMercadonaScraper(cfg.MercadonaPostalCode, cfg.ScraperUserAgent),
		scraper.NewCarrefourScraper(cfg.ScraperUserAgent),
		scraper.NewFroizScraper(cfg.ScraperUserAgent),
		scraper.NewGadisScraper(cfg.ScraperUserAgent),
		scraper.NewAlcampoScraper(cfg.ScraperUserAgent),
		scraper.NewEroskiScraper(cfg.ScraperUserAgent),
	)
	scheduler.Start(cfg.ScraperCron)
	defer scheduler.Stop()

	// Router
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.RequestLogger())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.AllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Request-ID", "Cache-Control"},
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/health", healthHandler.Health)
	r.GET("/livez", healthHandler.Live)

	api := r.Group("/api/v1")
	api.Use(middleware.GlobalRateLimiter(200, time.Minute))

	// Auth
	api.POST("/auth/login",
		middleware.StrictRateLimiter(10, 15*time.Minute),
		authHandler.Login,
	)
	api.POST("/auth/register",
		middleware.StrictRateLimiter(3, time.Hour),
		authHandler.Register,
	)

	// Protected
	auth := api.Group("")
	auth.Use(middleware.AuthRequired(authSvc))
	{
		auth.GET("/auth/me", authHandler.Me)
		auth.PATCH("/auth/me", authHandler.UpdateMe)

		// Catalog
		auth.GET("/categories", catalogHandler.ListCategories)
		auth.GET("/products", catalogHandler.ListProducts)
		auth.GET("/products/:id", catalogHandler.GetProduct)

		// Pricing
		auth.GET("/products/:id/prices", pricingHandler.GetPrices)
		auth.GET("/products/:id/prices/history", pricingHandler.GetPriceHistory)
		auth.GET("/cheapest", pricingHandler.GetCheapest)
		auth.GET("/preferences/supermarkets", pricingHandler.GetUserPrefs)
		auth.PUT("/preferences/supermarkets", pricingHandler.SetUserPrefs)

		// Shopping lists
		auth.GET("/lists", listHandler.GetLists)
		auth.POST("/lists", listHandler.CreateList)
		auth.GET("/lists/:id", listHandler.GetList)
		auth.PUT("/lists/:id", listHandler.UpdateList)
		auth.DELETE("/lists/:id", listHandler.DeleteList)
		auth.POST("/lists/join/:token", listHandler.JoinByToken)
		auth.POST("/lists/:id/items", listHandler.AddItem)
		auth.PUT("/lists/:id/items/:itemId", listHandler.UpdateItem)
		auth.DELETE("/lists/:id/items/:itemId", listHandler.DeleteItem)
		auth.GET("/lists/:id/optimize", listHandler.OptimizeList)

		// Admin
		admin := auth.Group("/admin")
		{
			admin.POST("/scrape/:name", func(c *gin.Context) {
				if cfg.AdminSecret == "" || c.GetHeader("X-Admin-Secret") != cfg.AdminSecret {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
					return
				}
				name := c.Param("name")
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
					defer cancel()
					_ = scheduler.RunOne(ctx, name)
				}()
				c.JSON(http.StatusAccepted, gin.H{"message": "scrape triggered", "scraper": name})
			})
		}
	}

	// SSE: EventSource cannot send custom headers — auth via ?token= query param
	api.GET("/lists/:id/events", middleware.SSEAuthRequired(authSvc), listHandler.StreamEvents)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server starting", slog.String("port", cfg.Port), slog.String("env", cfg.Env))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", slog.String("error", err.Error()))
	}
	slog.Info("server stopped")
}
