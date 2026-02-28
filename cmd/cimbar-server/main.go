// Package main provides the cimbar web server entry point.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/HirAhiraPpy/libcimbar/go/internal/server"
)

func main() {
	// Command line flags
	addr := flag.String("addr", ":8080", "HTTP listen address")
	outputDir := flag.String("output-dir", "/tmp/cimbar-downloads", "Directory to save decoded files")
	cacheDir := flag.String("cache-dir", "./data/cache", "Directory to cache user files")
	dbPath := flag.String("db-path", "./data/cimbar.db", "Path to SQLite database")
	mode := flag.String("mode", "Auto", "Cimbar decode mode (Auto, B, Bu, Bm, 4C)")
	workers := flag.Int("workers", 4, "Number of decoder workers")
	webDir := flag.String("web-dir", "./web/server", "Directory containing static web files")

	flag.Parse()

	// Validate output directory
	if *outputDir == "" {
		log.Fatal("output-dir is required")
	}

	// Ensure data directory exists
	dataDir := filepath.Dir(*dbPath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Create server with config
	cfg := server.ServerConfig{
		OutputDir: *outputDir,
		CacheDir:  *cacheDir,
		Mode:      *mode,
		Workers:   *workers,
		DBPath:    *dbPath,
	}
	srv, err := server.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Setup HTTP handlers
	fs := http.Dir(*webDir)
	mux := http.NewServeMux()

	// Static files
	mux.Handle("/", srv.StaticHandler(fs))

	// WebSocket endpoint
	mux.HandleFunc("/ws", srv.WSHandler)

	// Authentication APIs
	mux.HandleFunc("/api/login", srv.LoginHandler)
	mux.HandleFunc("/api/logout", srv.LogoutHandler)
	mux.HandleFunc("/api/session/validate", srv.ValidateSessionHandler)
	mux.HandleFunc("/api/user", srv.UserInfoHandler)

	// File management APIs
	mux.HandleFunc("/api/files", srv.FilesHandler)           // GET list, DELETE all
	mux.HandleFunc("/api/files/", srv.FileDetailHandler)     // GET/DELETE single file
	mux.HandleFunc("/api/download", srv.DownloadHandler)     // Download file
	mux.HandleFunc("/api/upload", srv.StreamDownloadHandler) // Upload file (from decoder)

	// Create HTTP server with timeouts
	httpServer := &http.Server{
		Addr:         *addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Channel to listen for interrupt signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		log.Printf("Starting cimbar web server on %s", *addr)
		log.Printf("Web UI: http://%s", *addr)
		log.Printf("Output directory: %s", *outputDir)
		log.Printf("Cache directory: %s", *cacheDir)
		log.Printf("Database path: %s", *dbPath)
		log.Printf("Press Ctrl+C to stop")

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-shutdown
	log.Println("\nReceived shutdown signal, stopping server...")

	// Close server (closes WebSocket connections)
	srv.Close()

	// Create a deadline for the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Graceful shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
