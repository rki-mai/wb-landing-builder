// main.go
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
	"github.com/rki-mai/wb-landing-builder/storage"
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

	authService := auth.NewAuthService(authRepository, cfg)

	authHandler := auth.NewAuthHandler(authService)

	authMiddleware := auth.AuthMiddleware(authService)

	mux := http.NewServeMux()

	authHandler.RegisterRoutes(mux, authMiddleware)
	draftHandler.RegisterRoutes(mux, authMiddleware)

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

	log.Println("Server exited properly")
}
