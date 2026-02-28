// Package server provides HTTP and WebSocket handlers for the cimbar web server.
package server

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/HirAhiraPpy/libcimbar/go/internal/decoder"
	"github.com/HirAhiraPpy/libcimbar/go/internal/session"

	"github.com/gorilla/websocket"
)

// CompletedFile represents a completed file download
type CompletedFile struct {
	Filename    string    `json:"filename"`
	FileSize    uint32    `json:"file_size"`
	Path        string    `json:"path"`
	CompletedAt time.Time `json:"completed_at"`
}

// DecodeResponse represents the response sent back to the client after decoding
type DecodeResponse struct {
	Type         string    `json:"type"`
	Success      bool      `json:"success,omitempty"`
	Error        string    `json:"error,omitempty"`
	Filename     string    `json:"filename,omitempty"`
	FileSize     uint32    `json:"file_size,omitempty"`
	BytesRecv    uint32    `json:"bytes_recv,omitempty"`
	BytesDecoded uint32    `json:"bytes_decoded,omitempty"`
	Progress     []float64 `json:"progress,omitempty"`
	Bytes        int       `json:"bytes,omitempty"`
	Extracted    bool      `json:"extracted,omitempty"`
	Failed       bool      `json:"failed,omitempty"`
	NoData       bool      `json:"nodata,omitempty"`
	IsDuplicate  bool      `json:"is_duplicate,omitempty"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	OutputDir   string
	CacheDir    string
	Mode        string
	Workers     int
	DBPath      string
}

// Server represents the cimbar web server with session management
type Server struct {
	outputDir      string
	cacheDir       string
	mode           string
	workers        int
	upgrader       websocket.Upgrader
	decoders       []*decoder.Decoder
	nextDec        int
	mu             sync.Mutex
	closeCh        chan struct{}
	wg             sync.WaitGroup
	sessionManager *session.SessionManager
}

// NewServer creates a new server instance with session management
func NewServer(cfg ServerConfig) (*Server, error) {
	// Ensure output directory exists
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cfg.CacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Initialize session database
	db, err := session.NewDatabase(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize session database: %w", err)
	}

	s := &Server{
		outputDir: cfg.OutputDir,
		cacheDir:  cfg.CacheDir,
		mode:      cfg.Mode,
		workers:   cfg.Workers,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024 * 1024,
			WriteBufferSize: 1024 * 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for local development
			},
		},
		decoders:       make([]*decoder.Decoder, cfg.Workers),
		nextDec:        0,
		closeCh:        make(chan struct{}),
		sessionManager: session.NewSessionManager(db),
	}

	// Initialize decoders
	for i := 0; i < cfg.Workers; i++ {
		s.decoders[i] = decoder.NewDecoder(cfg.Mode)
	}

	log.Printf("Server initialized with %d workers, mode: %s, output: %s, cache: %s",
		cfg.Workers, cfg.Mode, cfg.OutputDir, cfg.CacheDir)
	return s, nil
}

// Close gracefully shuts down the server
func (s *Server) Close() {
	log.Println("Closing server...")
	close(s.closeCh)
	s.wg.Wait()
	if s.sessionManager != nil {
		s.sessionManager.GetDatabase().Close()
	}
	log.Println("Server closed")
}

// SessionManager returns the session manager
func (s *Server) SessionManager() *session.SessionManager {
	return s.sessionManager
}

// StaticHandler serves static files from the web/server directory
func (s *Server) StaticHandler(fs http.FileSystem) http.Handler {
	return http.FileServer(fs)
}

// WSHandler handles WebSocket connections for video frame decoding
func (s *Server) WSHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("WebSocket request received from %s", r.RemoteAddr)

	// 1. Validate session token
	token := getSessionToken(r)
	if token == "" {
		log.Printf("WebSocket connection rejected: no session token")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sess, err := s.sessionManager.ValidateSession(token)
	if err != nil {
		log.Printf("WebSocket connection rejected: %v", err)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	log.Printf("Session validated for user %s (ID: %d)", sess.Email, sess.UserID)

	// 2. WebSocket upgrade
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("✓ Client connected: %s (user %s)", conn.RemoteAddr(), sess.Email)

	// 3. Kick previous session for this user (close old connection)
	if prevConn := s.sessionManager.RegisterConnection(sess.UserID, conn); prevConn != nil {
		log.Printf("Kicking previous session for user %d", sess.UserID)
		prevConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "logged in elsewhere"))
		prevConn.Close()
	}
	defer s.sessionManager.UnregisterConnection(sess.UserID)

	s.wg.Add(1)
	defer s.wg.Done()

	// 4. Get decoder for this connection
	s.mu.Lock()
	dec := s.decoders[s.nextDec]
	s.nextDec = (s.nextDec + 1) % len(s.decoders)
	s.mu.Unlock()

	// 5. Get cache directory for this session
	cacheDir, err := session.EnsureCacheDir(s.cacheDir, sess.ID)
	if err != nil {
		log.Printf("Failed to create cache dir: %v", err)
		s.sendResponse(conn, DecodeResponse{Type: "error", Error: "failed to initialize session"})
		return
	}

	// Track decode state
	var currentFileID uint32 = 0
	var frameCounter int = 0
	var currentFileHash string = ""
	var logTicker = time.NewTicker(time.Second * 10)
	defer logTicker.Stop()

	// Read loop
	for {
		// Check if server is shutting down
		select {
		case <-s.closeCh:
			log.Println("Server shutting down, closing WebSocket connection")
			return
		case <-logTicker.C:
			log.Printf("Frame stats: received=%d, current_file_id=%d, user=%d", frameCounter, currentFileID, sess.UserID)
		default:
		}

		// Set read deadline to 120 seconds
		conn.SetReadDeadline(time.Now().Add(time.Second * 120))

		// Read message
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle ping/message from client (keep-alive or control commands)
		if msgType == websocket.TextMessage {
			if len(data) > 0 && string(data[:1]) == "{" {
				var ctrlMsg struct {
					Type string `json:"type"`
				}
				if err := json.Unmarshal(data, &ctrlMsg); err == nil {
					switch ctrlMsg.Type {
					case "ping":
						continue
					case "reset_decoder":
						log.Println("Resetting decoder state for new file")
						currentFileID = 0
						currentFileHash = ""
						decoder.Reset()
						continue
					}
				}
			}
			continue
		}

		if msgType != websocket.BinaryMessage {
			continue
		}

		frameCounter++

		// Parse frame header
		if len(data) < 6 {
			s.sendResponse(conn, DecodeResponse{Type: "error", Error: "invalid frame format"})
			continue
		}

		format := decoder.ImageFormat(data[0])
		width := int(binary.LittleEndian.Uint16(data[2:4]))
		height := int(binary.LittleEndian.Uint16(data[4:6]))
		pixelData := data[6:]

		// Validate format
		var expectedSize int
		switch format {
		case decoder.FormatRGBA:
			expectedSize = width * height * 4
		case decoder.FormatRGB:
			expectedSize = width * height * 3
		case decoder.FormatNV12:
			expectedSize = width * height * 3 / 2
		case decoder.FormatI420:
			expectedSize = width * height * 3 / 2
		default:
			s.sendResponse(conn, DecodeResponse{Type: "error", Error: fmt.Sprintf("unknown format: %d", format)})
			continue
		}

		if len(pixelData) != expectedSize {
			log.Printf("Size mismatch: expected %d, got %d", expectedSize, len(pixelData))
		}

		// Step 1: Scan, extract and decode
		result, extractedData, err := dec.ScanExtractDecode(pixelData, width, height, format)
		if err != nil {
			log.Printf("Decode error: %v", err)
			s.sendResponse(conn, DecodeResponse{Type: "error", Error: err.Error()})
			continue
		}

		if frameCounter%10 == 0 {
			log.Printf("Frame %d: %dx%d, result=%d bytes, extracted=%v, failed=%v",
				frameCounter, width, height, result.Bytes, result.Extracted, result.Failed)
		}

		if result.Failed {
			s.sendResponse(conn, DecodeResponse{
				Type:      "decode",
				Extracted: false,
				Failed:    true,
			})
			continue
		}

		if !result.Extracted || result.Bytes == 0 {
			s.sendResponse(conn, DecodeResponse{
				Type:      "decode",
				Extracted: result.Extracted,
				NoData:    true,
			})
			continue
		}

		// Step 2: Fountain decode
		fountainResult, err := dec.FountainDecode(extractedData)
		if err != nil {
			s.sendResponse(conn, DecodeResponse{Type: "error", Error: err.Error()})
			continue
		}

		// Check if file is complete
		if fountainResult.FileID > 0 {
			newFileID := fountainResult.FileID
			if newFileID != currentFileID {
				currentFileID = newFileID
				s.handleFileComplete(conn, newFileID, sess.ID, cacheDir, currentFileHash)
			}
			continue
		}

		// Handle duplicate frame
		if fountainResult.IsDuplicate {
			s.sendResponse(conn, DecodeResponse{
				Type:        "decode",
				IsDuplicate: true,
				FileSize:    fountainResult.FileSize,
				BytesRecv:   fountainResult.BytesRecv,
				Progress:    fountainResult.Progress,
			})
			continue
		}

		// Send decode response with progress
		s.sendResponse(conn, DecodeResponse{
			Type:         "decode",
			Bytes:        result.Bytes,
			Extracted:    true,
			FileSize:     fountainResult.FileSize,
			BytesRecv:    fountainResult.BytesRecv,
			BytesDecoded: fountainResult.BytesRecv,
			Progress:     fountainResult.Progress,
		})
	}

	log.Printf("Client disconnected: %s (user %s)", conn.RemoteAddr(), sess.Email)
}

// handleFileComplete handles a completed file decode
func (s *Server) handleFileComplete(conn *websocket.Conn, fileID uint32, sessionID int64, cacheDir string, fileHash string) {
	log.Printf("=== File complete! FileID: %d, Session: %d ===", fileID, sessionID)

	filename, err := decoder.GetFilename(fileID)
	if err != nil {
		log.Printf("GetFilename error: %v", err)
		s.sendResponse(conn, DecodeResponse{Type: "error", Error: fmt.Sprintf("get filename: %v", err)})
		return
	}

	if filename == "" {
		filename = fmt.Sprintf("cimbar_file_%d", fileID)
		log.Printf("No filename in data, using: %s", filename)
	} else {
		log.Printf("Filename from data: %s", filename)
	}

	fileSize := decoder.GetFileSize(fileID)
	log.Printf("File size: %d bytes", fileSize)

	// Read decompressed data
	var allData []byte
	chunkCount := 0
	for {
		chunk, err := decoder.DecompressRead(fileID)
		if err != nil {
			log.Printf("DecompressRead error (chunk %d): %v", chunkCount, err)
			break
		}
		if len(chunk) == 0 {
			log.Printf("DecompressRead complete after %d chunks, total %d bytes", chunkCount, len(allData))
			break
		}
		allData = append(allData, chunk...)
		chunkCount++
		log.Printf("DecompressRead chunk %d: %d bytes (total: %d)", chunkCount, len(chunk), len(allData))
	}

	if len(allData) == 0 {
		log.Printf("ERROR: No data to write for fileID %d", fileID)
		s.sendResponse(conn, DecodeResponse{Type: "error", Error: "no data to write"})
		return
	}

	// Compute file hash if not provided
	if fileHash == "" {
		fileHash = session.ComputeFileHash(fmt.Sprintf("%s-%d-%d", filename, fileID, time.Now().UnixNano()))
	}

	// Save to cache directory for this session
	filePath := filepath.Join(cacheDir, fileHash)
	log.Printf("Writing file to: %s", filePath)
	if err := os.WriteFile(filePath, allData, 0644); err != nil {
		log.Printf("WriteFile error: %v", err)
		s.sendResponse(conn, DecodeResponse{Type: "error", Error: fmt.Sprintf("write file: %v", err)})
		return
	}

	log.Printf("=== File saved successfully: %s (%d bytes) ===", filePath, len(allData))

	// Save metadata to database
	db := s.sessionManager.GetDatabase()
	savedFileID, err := db.SaveFile(sessionID, fileHash, filename, filePath, int64(len(allData)))
	if err != nil {
		log.Printf("Failed to save file metadata: %v", err)
	} else {
		log.Printf("File metadata saved, DB ID: %d", savedFileID)

		// Mark file as complete with fountain info
		if err := db.MarkFileComplete(sessionID, fileHash, fileID, int64(fileSize), 0); err != nil {
			log.Printf("Failed to mark file complete: %v", err)
		}
	}

	// Notify client
	s.sendResponse(conn, DecodeResponse{
		Type:     "complete",
		Success:  true,
		Filename: filename,
		FileSize: fileSize,
	})
}

// sendResponse sends a JSON response to the client
func (s *Server) sendResponse(conn *websocket.Conn, resp DecodeResponse) {
	conn.WriteJSON(resp)
}
