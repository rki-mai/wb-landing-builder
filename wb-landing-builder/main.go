// Package main точка входа в приложение WB Landing Builder API.
//
// @title           WB Landing Builder API
// @version         1.0
// @description     API для управления черновиками лендингов и аутентификации пользователей.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /
// @schemes   http

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rki-mai/wb-landing-builder/auth"
	"github.com/rki-mai/wb-landing-builder/config"
	"github.com/rki-mai/wb-landing-builder/publishing"
	pubutils "github.com/rki-mai/wb-landing-builder/publishing/utils"
	pubworker "github.com/rki-mai/wb-landing-builder/publishing/worker"
	"github.com/rki-mai/wb-landing-builder/storage"

	_ "github.com/rki-mai/wb-landing-builder/docs"

	httpSwagger "github.com/swaggo/http-swagger/v2"
)

func main() {
	cfg := config.Load()

	log.Printf("Starting Landing Builder API on port: %s", cfg.Port)
	log.Printf("Environment: %s", cfg.Environment)

	log.Print(cfg.GetMongoURI(), " ", cfg.DBConfig.Database)

	draftRepository, err := storage.NewDraftRepository(cfg.GetMongoURI(), cfg.DBConfig.Database, cfg.DBConfig.TtlDays)
	if err != nil {
		log.Fatalf("Failed to init draft repository: %v", err)
	}

	authRepository, err := auth.NewAuthRepository(cfg.GetMongoURI(), cfg.DBConfig.Database)
	if err != nil {
		log.Fatalf("Failed to init auth repository: %v", err)
	}

	draftHandler, err := storage.NewDraftHandler(storage.NewDraftService(draftRepository, cfg), cfg)
	if err != nil {
		log.Fatalf("Draft handler creation failed: %v", err)
	}

	initCtx, initCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer initCancel()

	blobStorage, err := pubutils.NewS3BlobStorage(initCtx, cfg.S3)
	if err != nil {
		log.Fatalf("Failed to init blob storage: %v", err)
	}

	pubRepository, err := publishing.NewPublicationRepository(cfg.GetMongoURI(), cfg.DBConfig.Database, cfg.DBConfig.TtlDays)
	if err != nil {
		log.Fatalf("Failed to init publication repository: %v", err)
	}

	queue, err := pubutils.NewRabbitMQ(pubutils.RabbitMQConfig{
		URL:   cfg.RabbitMQ.URL,
		Queue: cfg.RabbitMQ.Queue,
	})
	if err != nil {
		log.Fatalf("Failed to init rabbitmq: %v", err)
	}

	renderer := pubutils.NewCLIRenderer(cfg.Publishing.CLIPath)
	pubDrafts := pubutils.NewStorageDraftReader(storage.NewDraftService(draftRepository, cfg))
	pubService := publishing.NewPublicationService(pubRepository, blobStorage, renderer, pubDrafts, queue)
	pubHandler := publishing.NewPublicationHandler(pubService)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	go func() {
		if err := pubworker.New(queue, pubService).Run(workerCtx); err != nil && err != context.Canceled {
			log.Printf("Publication worker stopped: %v", err)
		}
	}()

	authService := auth.NewAuthService(authRepository, cfg)

	authHandler := auth.NewAuthHandler(authService)

	authMiddleware := auth.AuthMiddleware(authService)

	mux := http.NewServeMux()

	authHandler.RegisterRoutes(mux, authMiddleware)
	draftHandler.RegisterRoutes(mux, authMiddleware)
	pubHandler.RegisterRoutes(mux, authMiddleware)

	mux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server is running on http://localhost:%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	workerCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	if err := draftRepository.Close(ctx); err != nil {
		log.Printf("Failed to close draft repository: %v", err)
	}

	if err := authRepository.Close(ctx); err != nil {
		log.Printf("Failed to close auth repository: %v", err)
	}

	if err := pubRepository.Close(ctx); err != nil {
		log.Printf("Failed to close publication repository: %v", err)
	}

	if err := queue.Close(); err != nil {
		log.Printf("Failed to close rabbitmq: %v", err)
	}

	log.Println("Server exited properly")
}
