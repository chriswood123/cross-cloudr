package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chriswood123/cross-cloudr/backend/internal/api"
	"github.com/chriswood123/cross-cloudr/backend/internal/auth"
	"github.com/chriswood123/cross-cloudr/backend/internal/config"
	"github.com/chriswood123/cross-cloudr/backend/internal/db"
	"github.com/chriswood123/cross-cloudr/backend/internal/encrypt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	encryptor, err := encrypt.New(cfg.EncryptionKey)
	if err != nil {
		log.Fatalf("failed to create encryptor: %v", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer database.Pool.Close()

	if err := database.Migrate(); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPasswordHash), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("failed to hash admin password: %v", err)
	}

	repo := db.NewRepository(database.Pool, encryptor)
	if err := repo.EnsureAdmin(context.Background(), cfg.AdminUsername, string(passwordHash)); err != nil {
		log.Fatalf("failed to ensure admin user: %v", err)
	}

	authService := auth.NewServiceWithHash(cfg.SessionSecret, cfg.AdminUsername, string(passwordHash))
	handler := api.NewHandler(repo, authService)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	apiGroup := r.Group("/api")
	{
		apiGroup.POST("/login", handler.Login)
		apiGroup.GET("/me", authService.Middleware(), handler.Me)
		apiGroup.GET("/credentials", authService.Middleware(), handler.ListCredentials)
		apiGroup.GET("/credentials/:provider", authService.Middleware(), handler.GetCredentials)
		apiGroup.POST("/credentials/:provider", authService.Middleware(), handler.SaveCredentials)
		apiGroup.PUT("/credentials/:provider", authService.Middleware(), handler.SaveCredentials)
		apiGroup.DELETE("/credentials/:provider", authService.Middleware(), handler.DeleteCredentials)
		apiGroup.POST("/credentials/:provider/test", authService.Middleware(), handler.TestCredentials)
		apiGroup.GET("/vms", authService.Middleware(), handler.ListVMs)
	}

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to start server: %v", err)
		}
	}()

	log.Printf("server listening on port %s", cfg.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server forced to shutdown: %v", err)
	}
}
