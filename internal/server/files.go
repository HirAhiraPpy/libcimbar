package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/HirAhiraPpy/libcimbar/go/internal/session"
)

// FileResponse represents a file in the API response
type FileResponse struct {
	FileHash         string `json:"file_hash"`
	OriginalFilename string `json:"original_filename"`
	FileSize         int64  `json:"file_size"`
	CreatedAt        string `json:"created_at"`
	IsComplete       bool   `json:"is_complete"`
}

// FilesListResponse represents a list of files
type FilesListResponse struct {
	Files []FileResponse `json:"files"`
}

// DeleteResponse represents a delete response
type DeleteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// FilesHandler handles file list requests
func (s *Server) FilesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := getSessionToken(r)
	if token == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	sess, err := s.sessionManager.ValidateSession(token)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Get files for this session
	files, err := s.sessionManager.GetDatabase().GetSessionFiles(sess.ID)
	if err != nil {
		log.Printf("Failed to get files for session %d: %v", sess.ID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to get files"})
		return
	}

	// Convert to response format
	resp := FilesListResponse{
		Files: make([]FileResponse, 0, len(files)),
	}
	for _, f := range files {
		resp.Files = append(resp.Files, FileResponse{
			FileHash:         f.FileHash,
			OriginalFilename: f.OriginalFilename,
			FileSize:         f.FileSize,
			CreatedAt:        f.CreatedAt.Format(time.RFC3339),
			IsComplete:       f.IsComplete,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// DeleteFileHandler handles single file deletion
func (s *Server) DeleteFileHandler(w http.ResponseWriter, r *http.Request, fileHash string) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := getSessionToken(r)
	if token == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(DeleteResponse{Success: false, Error: "unauthorized"})
		return
	}

	sess, err := s.sessionManager.ValidateSession(token)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(DeleteResponse{Success: false, Error: err.Error()})
		return
	}

	// Get file to verify it exists and belongs to this session
	if _, err := s.sessionManager.GetDatabase().GetFileByHash(sess.ID, fileHash); err != nil {
		if err == session.ErrFileNotFound {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(DeleteResponse{Success: false, Error: "file not found"})
			return
		}
		log.Printf("Failed to get file: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(DeleteResponse{Success: false, Error: "failed to get file"})
		return
	}

	// Soft delete
	if err := s.sessionManager.GetDatabase().SoftDeleteFile(sess.ID, fileHash); err != nil {
		log.Printf("Failed to delete file: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(DeleteResponse{Success: false, Error: "failed to delete file"})
		return
	}

	// Optionally remove physical file (for now, keep it)
	// os.Remove(file.FilePath)

	log.Printf("File deleted: %s (session %d)", fileHash, sess.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DeleteResponse{Success: true, Message: "file deleted"})
}

// DeleteAllFilesHandler handles bulk file deletion
func (s *Server) DeleteAllFilesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := getSessionToken(r)
	if token == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(DeleteResponse{Success: false, Error: "unauthorized"})
		return
	}

	sess, err := s.sessionManager.ValidateSession(token)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(DeleteResponse{Success: false, Error: err.Error()})
		return
	}

	count, err := s.sessionManager.GetDatabase().DeleteAllSessionFiles(sess.ID)
	if err != nil {
		log.Printf("Failed to delete all files: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(DeleteResponse{Success: false, Error: "failed to delete files"})
		return
	}

	log.Printf("Deleted %d files for session %d", count, sess.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DeleteResponse{Success: true, Message: fmt.Sprintf("deleted %d files", count)})
}

// DownloadHandler serves file downloads
func (s *Server) DownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileHash := r.URL.Query().Get("hash")
	if fileHash == "" {
		http.Error(w, "hash parameter required", http.StatusBadRequest)
		return
	}

	token := getSessionToken(r)
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sess, err := s.sessionManager.ValidateSession(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Get file from database
	file, err := s.sessionManager.GetDatabase().GetFileByHash(sess.ID, fileHash)
	if err != nil {
		if err == session.ErrFileNotFound {
			http.Error(w, "file not found", http.StatusNotFound)
			return
		}
		log.Printf("Failed to get file: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Check if file was deleted
	if !file.DeletedAt.IsZero() {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	// Check if file exists on disk
	if _, err := os.Stat(file.FilePath); os.IsNotExist(err) {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	// Serve file
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", file.OriginalFilename))
	w.Header().Set("Content-Type", "application/octet-stream")

	http.ServeFile(w, r, file.FilePath)
}

// StreamDownloadHandler handles file upload from stdin (for decoder)
func (s *Server) StreamDownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fileHash := r.URL.Query().Get("hash")
	sessionID := r.URL.Query().Get("session_id")
	filename := r.URL.Query().Get("filename")

	if fileHash == "" || sessionID == "" || filename == "" {
		http.Error(w, "hash, session_id, and filename required", http.StatusBadRequest)
		return
	}

	// Parse session ID
	var sessID int64
	if _, err := fmt.Sscanf(sessionID, "%d", &sessID); err != nil {
		http.Error(w, "invalid session_id", http.StatusBadRequest)
		return
	}

	// Ensure cache directory exists
	cacheDir, err := session.EnsureCacheDir(s.cacheDir, sessID)
	if err != nil {
		log.Printf("Failed to create cache dir: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Save file
	filePath := filepath.Join(cacheDir, fileHash)
	outFile, err := os.Create(filePath)
	if err != nil {
		log.Printf("Failed to create file: %v", err)
		http.Error(w, "failed to create file", http.StatusInternalServerError)
		return
	}
	defer outFile.Close()

	// Read body and write to file
	bytesWritten, err := io.Copy(outFile, r.Body)
	if err != nil {
		log.Printf("Failed to write file: %v", err)
		http.Error(w, "failed to write file", http.StatusInternalServerError)
		return
	}

	// Save metadata to database
	db := s.sessionManager.GetDatabase()
	fileID, err := db.SaveFile(sessID, fileHash, filename, filePath, bytesWritten)
	if err != nil {
		log.Printf("Failed to save file metadata: %v", err)
		http.Error(w, "failed to save metadata", http.StatusInternalServerError)
		return
	}

	log.Printf("File saved: %s (%d bytes), fileID: %d", filename, bytesWritten, fileID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"file_id":   fileID,
		"file_hash": fileHash,
	})
}

// FileDetailHandler handles requests for individual files
func (s *Server) FileDetailHandler(w http.ResponseWriter, r *http.Request) {
	// Extract file hash from URL path
	// Path format: /api/files/{hash}
	path := r.URL.Path
	fileHash := strings.TrimPrefix(path, "/api/files/")

	if fileHash == "" {
		// Empty hash means bulk operation
		if r.Method == http.MethodDelete {
			s.DeleteAllFilesHandler(w, r)
			return
		}
		s.FilesHandler(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// Serve file download
		s.DownloadHandler(w, r)
	case http.MethodDelete:
		// Delete single file
		s.DeleteFileHandler(w, r, fileHash)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
