// Package main provides the cimbar web server entry point.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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
	http.Handle("/", srv.StaticHandler(fs))
	http.HandleFunc("/ws", srv.WSHandler)

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down...")
	}()

	// Start server
	log.Printf("Starting cimbar web server on %s", *addr)
	log.Printf("Web UI: http://localhost%s", *addr)
	log.Printf("Output directory: %s", *outputDir)

	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
