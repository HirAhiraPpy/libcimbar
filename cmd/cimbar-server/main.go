// Package main provides the cimbar web server entry point.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HirAhiraPpy/libcimbar/go/internal/server"
)

func main() {
	// Command line flags
	addr := flag.String("addr", ":8080", "HTTP listen address")
	outputDir := flag.String("output-dir", "/tmp/cimbar-downloads", "Directory to save decoded files")
	mode := flag.String("mode", "Auto", "Cimbar decode mode (Auto, B, Bu, Bm, 4C)")
	workers := flag.Int("workers", 4, "Number of decoder workers")
	webDir := flag.String("web-dir", "./web/server", "Directory containing static web files")

	flag.Parse()

	// Validate output directory
	if *outputDir == "" {
		log.Fatal("output-dir is required")
	}

	// Create server
	srv, err := server.NewServer(*outputDir, *mode, *workers)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Setup HTTP handlers
	fs := http.Dir(*webDir)
	mux := http.NewServeMux()
	mux.Handle("/", srv.StaticHandler(fs))
	mux.HandleFunc("/ws", srv.WSHandler)
	mux.HandleFunc("/api/files", srv.FilesHandler)
	mux.HandleFunc("/api/download", srv.DownloadHandler)

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
