package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/rki-mai/wb-landing-builder/auth"
	config "github.com/rki-mai/wb-landing-builder/configs"
	"github.com/rki-mai/wb-landing-builder/storage"
)

func main() {
	cfg := config.Load()

	log.Printf("Starting Landing Builder API on port: %s", cfg.Port)
	log.Printf("Environment: %s", cfg.Environment)

	draftRepository, err := storage.NewDraftRepository(
		cfg.GetMongoURI(),
		cfg.DBConfig.Database,
		cfg.DBConfig.TtlDays,
	)
	if err != nil {
		log.Fatalf("Failed to init draft repository: %v", err)
	}

	authRepository, err := auth.NewAuthRepository(
		cfg.GetMongoURI(),
		cfg.DBConfig.Database,
	)
	if err != nil {
		log.Fatalf("Failed to init auth repository: %v", err)
	}

	authService := auth.NewAuthService(authRepository, cfg)
	draftService := storage.NewDraftService(draftRepository, cfg)

	authHandler := auth.NewAuthHandler(authService)
	draftHandler, err := storage.NewDraftHandler(draftService, cfg)
	if err != nil {
		log.Fatalf("Draft handler creation failed: %v", err)
	}

	router := chi.NewMux()

	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	router.Use(auth.AuthMiddleware(authService))

	api := humachi.New(router, config.GetHumaConfig(cfg.ServerUrl+":"+cfg.Port))

	authHandler.RegisterRoutes(api)
	draftHandler.RegisterRoutes(api)

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Server is running on http://localhost:%s", cfg.Port)
		log.Printf("Huma UI: http://localhost:%s/docs", cfg.Port)
		log.Printf("OpenAPI JSON: http://localhost:%s/openapi.json", cfg.Port)
		log.Printf("Health check: http://localhost:%s/health", cfg.Port)

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
		log.Printf("Server forced to shutdown: %v", err)
	}

	if err := draftRepository.Close(ctx); err != nil {
		log.Printf("Failed to close draft repository: %v", err)
	}

	if err := authRepository.Close(ctx); err != nil {
		log.Printf("Failed to close auth repository: %v", err)
	}

	log.Println("Server exited properly")
}
