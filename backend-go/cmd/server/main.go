package main

import (
	"context"
	"log"
	"net/http"

	"dataguardian/backend-go/internal/auth"
	"dataguardian/backend-go/internal/config"
	"dataguardian/backend-go/internal/db"
	"dataguardian/backend-go/internal/httpapi"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	store, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database error: %v", err)
	}
	defer store.Close()

	if err := store.Init(); err != nil {
		log.Fatalf("database initialization error: %v", err)
	}
	if cfg.IsDevOrTest() {
		adminHash, err := auth.HashPassword("admin123")
		if err != nil {
			log.Fatalf("local user bootstrap error: %v", err)
		}
		testHash, err := auth.HashPassword("test123")
		if err != nil {
			log.Fatalf("local user bootstrap error: %v", err)
		}
		if err := store.EnsureLocalUser(context.Background(), "admin", adminHash, true); err != nil {
			log.Fatalf("local user bootstrap error: %v", err)
		}
		if err := store.EnsureLocalUser(context.Background(), "test", testHash, false); err != nil {
			log.Fatalf("local user bootstrap error: %v", err)
		}
	}

	server := &http.Server{
		Addr:         cfg.Addr,
		Handler:      httpapi.NewRouter(cfg, store),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	log.Printf("DataGuardian Go backend listening on %s", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
