package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

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
	if err := ensureStorageDirectory(cfg.StorageDir); err != nil {
		log.Fatalf("storage configuration error: %v", err)
	}
	log.Printf("storage directory configured: %s", cfg.StorageDir)
	if cfg.StorageOrphanRetention() <= 0 {
		log.Print("storage orphan cleanup disabled")
	} else {
		log.Printf("storage orphan cleanup retention: %s", cfg.StorageOrphanRetention())
	}
	if removed, err := cleanupOrphanedStoredFiles(context.Background(), store, cfg.StorageDir, cfg.StorageOrphanRetention()); err != nil {
		log.Printf("storage cleanup skipped: %v", err)
	} else if removed > 0 {
		log.Printf("storage cleanup removed %d orphaned file(s)", removed)
	}
	if cfg.IsDevOrTest() {
		log.Print("development demo users enabled")
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

func ensureStorageDirectory(storageDir string) error {
	if err := os.MkdirAll(storageDir, 0o700); err != nil {
		return err
	}
	info, err := os.Stat(storageDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", storageDir)
	}
	return nil
}

func cleanupOrphanedStoredFiles(ctx context.Context, store *db.Store, storageDir string, retention time.Duration) (int, error) {
	if retention <= 0 {
		return 0, nil
	}
	root, err := filepath.Abs(storageDir)
	if err != nil {
		return 0, err
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	references, err := store.StoredFileReferences(ctx)
	if err != nil {
		return 0, err
	}
	referenced := map[string]bool{}
	for _, reference := range references {
		absolute, err := filepath.Abs(reference)
		if err == nil {
			referenced[absolute] = true
		}
	}

	cutoff := time.Now().Add(-retention)
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if referenced[path] {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.ModTime().After(cutoff) {
			continue
		}
		if err := os.Remove(path); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}
