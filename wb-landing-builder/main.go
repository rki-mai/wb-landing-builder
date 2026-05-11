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

	draftConfig "github.com/rki-mai/wb-landing-builder/storage/config"
	draftHandler "github.com/rki-mai/wb-landing-builder/storage/handler"
	draftRepository "github.com/rki-mai/wb-landing-builder/storage/repository"
	draftService "github.com/rki-mai/wb-landing-builder/storage/service"

	authHandler "github.com/rki-mai/wb-landing-builder/auth/handler"
	authMiddleware "github.com/rki-mai/wb-landing-builder/auth/middleware"
	authRepository "github.com/rki-mai/wb-landing-builder/auth/repository"
	authService "github.com/rki-mai/wb-landing-builder/auth/service"
	authConfig "github.com/rki-mai/wb-landing-builder/config"
)

func main() {
	cfg := draftConfig.Load()
	authCfg := authConfig.Load()

	log.Printf("Starting Landing Builder API on port: %s", cfg.Port)
	log.Printf("Environment: %s", cfg.Environment)

	log.Print(cfg.GetMongoURI(), " ", cfg.DBConfig.Database)

	draftRepo, err := draftRepository.NewDraftRepository(cfg.GetMongoURI(), cfg.DBConfig.Database, cfg.DBConfig.TtlDays)
	if err != nil {
		log.Fatalf("Failed to init draft repository: %v", err)
	}

	authRepo, err := authRepository.NewAuthRepository(authCfg.GetMongoURI(), authCfg.DBConfig.Database)
	if err != nil {
		log.Fatalf("Failed to init auth repository: %v", err)
	}

	draftH, err := draftHandler.NewHandler(draftService.NewDraftService(draftRepo, cfg), cfg)
	if err != nil {
		log.Fatalf("Draft handler creation failed: %v", err)
	}

	authSvc := authService.NewAuthService(authRepo, authCfg)

	authH := authHandler.NewHandler(authSvc)

	authMV := authMiddleware.AuthMiddleware(authSvc)

	mux := http.NewServeMux()

	authH.RegisterRoutes(mux, authMV)
	draftH.RegisterRoutes(mux, authMV)

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

	if err := draftRepo.Close(ctx); err != nil {
		log.Printf("Failed to close draft repository: %v", err)
	}

	if err := authRepo.Close(ctx); err != nil {
		log.Printf("Failed to close auth repository: %v", err)
	}

	log.Println("Server exited properly")
}
