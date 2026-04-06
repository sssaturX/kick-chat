package main

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"license-server/internal/config"
	"license-server/internal/database"
	"license-server/internal/handler"
	"license-server/internal/logbuffer"
	"license-server/internal/middleware"
	"license-server/internal/models"
	"license-server/internal/repository"
	"license-server/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

//go:embed static
var staticFS embed.FS

func main() {
	indexHTML, _ := fs.ReadFile(staticFS, "static/index.html")
	loginHTML, _ := fs.ReadFile(staticFS, "static/admin_login.html")
	dashHTML, _ := fs.ReadFile(staticFS, "static/admin_dashboard.html")
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("config load", zap.Error(err))
	}

	ctx := context.Background()
	db, err := database.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("database", zap.Error(err))
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := db.AutoMigrate(&models.License{}, &models.Activation{}); err != nil {
		logger.Fatal("migrate", zap.Error(err))
	}

	redisClient, err := database.NewRedis(cfg.RedisURL)
	if err != nil {
		logger.Fatal("redis", zap.Error(err))
	}
	defer redisClient.Close()

	licRepo := repository.NewLicenseRepository(db)
	actRepo := repository.NewActivationRepository(db)
	licService := service.NewLicenseService(licRepo, actRepo, redisClient, cfg.HMACSecret, logger)
	licHandler := handler.NewLicenseHandler(licService, cfg.HMACSecret, logger)
	adminUI := handler.NewAdminUIHandler(licService, redisClient, cfg.AdminAPIKey, cfg.AdminSessionTTL, cfg.AdminCookieSecure, logger)

	rateLimiter := middleware.NewRateLimiter(redisClient, cfg.RateLimitRPS, logger)
	adminAuth := middleware.AdminAuth(cfg.AdminAPIKey, redisClient)

	logBuf := logbuffer.New(500)

	router := gin.New()
	router.Use(gin.Recovery(), middleware.RequestID(), middleware.RequestLogToBuffer(logBuf), middleware.Logger(logger))
	router.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/admin/dashboard")
	})
	router.GET("/dev", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})
	router.GET("/index.html", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/dev")
	})

	router.GET("/admin", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/admin/dashboard")
	})
	router.GET("/admin/login", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", loginHTML)
	})
	router.GET("/admin/dashboard", middleware.AdminHTMLAuth(cfg.AdminAPIKey, redisClient), func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", dashHTML)
	})

	sessLogin := router.Group("/admin/api/session")
	sessLogin.Use(middleware.LoginRateLimit(redisClient, 5, time.Minute, logger))
	sessLogin.POST("/login", adminUI.Login)

	api := router.Group("")
	api.Use(rateLimiter.Middleware())
	{
		api.POST("/activate", licHandler.Activate)
		api.POST("/license/refresh", licHandler.Refresh)
		api.POST("/validate", licHandler.Validate)
	}

	if cfg.DownloadFilePath != "" {
		dh := handler.NewDownloadHandler(licService, redisClient, cfg.DownloadFilePath, cfg.DownloadFileName, logger)
		router.GET("/download", dh.PortalPage)
		dl := router.Group("/download")
		dl.Use(rateLimiter.Middleware())
		dl.POST("/verify", dh.Verify)
		router.GET("/download/file", dh.ServeFile)
	}

	admin := router.Group("/admin")
	admin.Use(adminAuth)
	admin.Use(rateLimiter.Middleware())
	{
		admin.POST("/api/session/logout", adminUI.Logout)
		admin.GET("/api/licenses", adminUI.ListLicenses)
		admin.POST("/delete", adminUI.DeleteLicense)
		admin.GET("/logs", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"lines": logBuf.Lines()})
		})
		admin.POST("/revoke", licHandler.AdminRevoke)
		admin.POST("/activate", licHandler.AdminActivate)
		admin.POST("/licenses", licHandler.AdminCreate)
	}

	srv := &http.Server{Addr: "0.0.0.0:" + cfg.Port, Handler: router}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("http server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown", zap.Error(err))
	}
	logger.Info("server stopped")
}
