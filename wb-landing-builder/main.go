// main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rki-mai/wb-landing-builder/storage/config"
	"github.com/rki-mai/wb-landing-builder/storage/handler"
	"github.com/rki-mai/wb-landing-builder/storage/repository"
	"github.com/rki-mai/wb-landing-builder/storage/service"
)

func main() {
	cfg := config.Load()

	log.Printf("Starting Draft Component API on port: %s", cfg.Port)
	log.Printf("Environment: %s", cfg.Environment)

	log.Print(cfg.GetMongoURI(), " ", cfg.DBConfig.Database)

	repository, err := repository.NewDraftRepository(cfg.GetMongoURI(), cfg.DBConfig.Database, cfg.DBConfig.TtlDays)
	if err != nil {
		log.Fatalf("Failed to connect to db: %v", err)
	}

	handler, err := handler.NewHandler(service.NewDraftService(repository, cfg), cfg)
	if err != nil {
		fmt.Printf("handler creation failed: %v", err)
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

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

	log.Println("Server exited properly")
}
