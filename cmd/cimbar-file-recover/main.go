// Package main provides a file recovery tool for cimbar server.
// This tool allows administrators to query the database and recover
// decoded files for any user to a specified location.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// FileWithUser represents a file with associated user information
type FileWithUser struct {
	ID               int64
	SessionID        int64
	FileHash         string
	OriginalFilename string
	FileSize         int64
	FilePath         string
	CreatedAt        time.Time
	IsComplete       bool
	DeletedAt        time.Time
	FountainFileID   uint32
	FountainFileSize int64
	BlocksRequired   int
	Email            string
}

func main() {
	// Command line flags
	dbPath := flag.String("db", "./data/cimbar.db", "Path to SQLite database")
	_ = flag.String("cache", "./data/cache", "Path to cache directory")
	outputDir := flag.String("output", "./recovered", "Output directory for recovered files")
	email := flag.String("email", "", "Filter by user email (optional)")
	fileHash := flag.String("hash", "", "Filter by file hash (optional)")
	listOnly := flag.Bool("list", false, "Only list files, don't copy them")
	help := flag.Bool("help", false, "Show help message")

	flag.Parse()

	if *help {
		showHelp()
		return
	}

	// Validate database path
	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		log.Fatalf("Database not found: %s", *dbPath)
	}

	// Open database
	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create output directory
	if !*listOnly {
		if err := os.MkdirAll(*outputDir, 0755); err != nil {
			log.Fatalf("Failed to create output directory: %v", err)
		}
	}

	// Query files
	files, err := queryFiles(db, *email, *fileHash)
	if err != nil {
		log.Fatalf("Failed to query files: %v", err)
	}

	if len(files) == 0 {
		fmt.Println("No files found matching the criteria.")
		return
	}

	// Display files
	fmt.Printf("\nFound %d file(s):\n\n", len(files))
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Printf("%-4s | %-25s | %-35s | %-20s | %-10s | %s\n",
		"ID", "Email", "Filename", "Hash", "Size", "Status")
	fmt.Println("--------------------------------------------------------------------------------")

	for i, f := range files {
		status := "complete"
		if !f.DeletedAt.IsZero() && f.DeletedAt.After(f.CreatedAt) {
			status = "deleted"
		}
		if !f.IsComplete {
			status = "incomplete"
		}

		email := f.Email
		if len(email) > 25 {
			email = email[:22] + "..."
		}
		filename := f.OriginalFilename
		if len(filename) > 35 {
			filename = filename[:32] + "..."
		}

		fmt.Printf("%-4d | %-25s | %-35s | %-20s | %-10d | %s\n",
			i+1, email, filename, f.FileHash, f.FileSize, status)
	}
	fmt.Println("--------------------------------------------------------------------------------")

	if *listOnly {
		return
	}

	// Recover files
	fmt.Println("\nRecovering files...")

	recoveredCount := 0
	skippedCount := 0
	errorCount := 0

	for _, f := range files {
		if !f.DeletedAt.IsZero() && f.DeletedAt.After(f.CreatedAt) {
			fmt.Printf("  [SKIP] %s (deleted)\n", f.OriginalFilename)
			skippedCount++
			continue
		}

		if !f.IsComplete {
			fmt.Printf("  [SKIP] %s (incomplete)\n", f.OriginalFilename)
			skippedCount++
			continue
		}

		// Check if source file exists
		if _, err := os.Stat(f.FilePath); os.IsNotExist(err) {
			fmt.Printf("  [ERROR] %s (source file not found: %s)\n", f.OriginalFilename, f.FilePath)
			errorCount++
			continue
		}

		// Create user subdirectory in output
		userDir := filepath.Join(*outputDir, sanitizeEmail(f.Email))
		if err := os.MkdirAll(userDir, 0755); err != nil {
			fmt.Printf("  [ERROR] Failed to create directory for %s: %v\n", f.Email, err)
			errorCount++
			continue
		}

		// Copy file
		outputPath := filepath.Join(userDir, sanitizeFilename(f.OriginalFilename))

		// Handle duplicate filenames
		if _, err := os.Stat(outputPath); err == nil {
			base := filepath.Base(f.OriginalFilename)
			ext := filepath.Ext(base)
			name := base[:len(base)-len(ext)]
			outputPath = filepath.Join(userDir, fmt.Sprintf("%s_%s%s", name, f.FileHash[:8], ext))
		}

		if err := copyFile(f.FilePath, outputPath); err != nil {
			fmt.Printf("  [ERROR] Failed to copy %s: %v\n", f.OriginalFilename, err)
			errorCount++
			continue
		}

		fmt.Printf("  [OK] %s -> %s\n", f.OriginalFilename, outputPath)
		recoveredCount++
	}

	fmt.Println("\n--------------------------------------------------------------------------------")
	fmt.Printf("Recovery complete: %d recovered, %d skipped, %d errors\n",
		recoveredCount, skippedCount, errorCount)
}

// queryFiles queries files from the database
func queryFiles(db *sql.DB, email, fileHash string) ([]*FileWithUser, error) {
	var query string
	var args []interface{}

	if email != "" && fileHash != "" {
		query = `
			SELECT f.id, f.session_id, f.file_hash, f.original_filename, f.file_size,
			       f.file_path, f.created_at, f.is_complete, COALESCE(f.deleted_at, ''),
			       COALESCE(f.fountain_file_id, 0), COALESCE(f.fountain_file_size, 0),
			       COALESCE(f.blocks_required, 0),
			       u.email
			FROM files f
			JOIN sessions s ON f.session_id = s.id
			JOIN users u ON s.user_id = u.id
			WHERE u.email = ? AND f.file_hash = ?
		`
		args = []interface{}{email, fileHash}
	} else if email != "" {
		query = `
			SELECT f.id, f.session_id, f.file_hash, f.original_filename, f.file_size,
			       f.file_path, f.created_at, f.is_complete, COALESCE(f.deleted_at, ''),
			       COALESCE(f.fountain_file_id, 0), COALESCE(f.fountain_file_size, 0),
			       COALESCE(f.blocks_required, 0),
			       u.email
			FROM files f
			JOIN sessions s ON f.session_id = s.id
			JOIN users u ON s.user_id = u.id
			WHERE u.email = ?
			ORDER BY f.created_at DESC
		`
		args = []interface{}{email}
	} else if fileHash != "" {
		query = `
			SELECT f.id, f.session_id, f.file_hash, f.original_filename, f.file_size,
			       f.file_path, f.created_at, f.is_complete, COALESCE(f.deleted_at, ''),
			       COALESCE(f.fountain_file_id, 0), COALESCE(f.fountain_file_size, 0),
			       COALESCE(f.blocks_required, 0),
			       u.email
			FROM files f
			JOIN sessions s ON f.session_id = s.id
			JOIN users u ON s.user_id = u.id
			WHERE f.file_hash = ?
			ORDER BY f.created_at DESC
		`
		args = []interface{}{fileHash}
	} else {
		query = `
			SELECT f.id, f.session_id, f.file_hash, f.original_filename, f.file_size,
			       f.file_path, f.created_at, f.is_complete, COALESCE(f.deleted_at, ''),
			       COALESCE(f.fountain_file_id, 0), COALESCE(f.fountain_file_size, 0),
			       COALESCE(f.blocks_required, 0),
			       u.email
			FROM files f
			JOIN sessions s ON f.session_id = s.id
			JOIN users u ON s.user_id = u.id
			ORDER BY f.created_at DESC
		`
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*FileWithUser
	for rows.Next() {
		var f FileWithUser
		var deletedAt string

		err := rows.Scan(
			&f.ID, &f.SessionID, &f.FileHash, &f.OriginalFilename, &f.FileSize,
			&f.FilePath, &f.CreatedAt, &f.IsComplete, &deletedAt,
			&f.FountainFileID, &f.FountainFileSize, &f.BlocksRequired,
			&f.Email,
		)
		if err != nil {
			return nil, err
		}

		if deletedAt != "" {
			f.DeletedAt, _ = time.Parse("2006-01-02 15:04:05", deletedAt)
		}

		files = append(files, &f)
	}

	return files, rows.Err()
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// sanitizeEmail creates a safe directory name from email
func sanitizeEmail(email string) string {
	// Replace problematic characters
	result := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		"..", "_",
	).Replace(email)
	return result
}

// sanitizeFilename creates a safe filename
func sanitizeFilename(filename string) string {
	// Remove path components and replace problematic characters
	base := filepath.Base(filename)
	result := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		"..", "_",
	).Replace(base)
	return result
}

// showHelp displays help message
func showHelp() {
	fmt.Println(`Cimbar File Recovery Tool

Usage: cimbar-file-recover [options]

This tool queries the cimbar server database and recovers decoded files
to a specified location.

Options:
  -db <path>       Path to SQLite database (default: ./data/cimbar.db)
  -cache <path>    Path to cache directory (default: ./data/cache)
  -output <dir>    Output directory for recovered files (default: ./recovered)
  -email <email>   Filter by user email (optional)
  -hash <hash>     Filter by file hash (optional)
  -list            Only list files, don't copy them
  -help            Show this help message

Examples:
  # List all files in the database
  cimbar-file-recover -list

  # List files for a specific user
  cimbar-file-recover -list -email user@example.com

  # Recover all files to ./recovered
  cimbar-file-recover

  # Recover files for a specific user
  cimbar-file-recover -email user@example.com

  # Recover a specific file by hash
  cimbar-file-recover -hash a3f5d8c2e1b4f6a7

  # Recover to a custom output directory
  cimbar-file-recover -output /path/to/output

File Recovery Logic:
  - Files are organized by user email in subdirectories
  - Deleted or incomplete files are skipped
  - Duplicate filenames are handled by appending hash prefix
  - Original filenames are preserved when possible
`)
}
