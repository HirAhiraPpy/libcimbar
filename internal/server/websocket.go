// Package server provides HTTP and WebSocket handlers for the cimbar web server.
package server

import (
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/HirAhiraPpy/libcimbar/go/internal/decoder"

	"github.com/gorilla/websocket"
)

// DecodeResponse represents the response sent back to the client after decoding
type DecodeResponse struct {
	Type      string `json:"type"`
	Success   bool   `json:"success,omitempty"`
	Error     string `json:"error,omitempty"`
	Filename  string `json:"filename,omitempty"`
	FileSize  uint32 `json:"file_size,omitempty"`
	Progress  []int  `json:"progress,omitempty"`
	Bytes     int    `json:"bytes,omitempty"`
	Extracted bool   `json:"extracted,omitempty"`
	Failed    bool   `json:"failed,omitempty"`
	NoData    bool   `json:"nodata,omitempty"`
}

// Server represents the cimbar web server
type Server struct {
	outputDir string
	mode      string
	workers   int
	upgrader  websocket.Upgrader
	decoders  []*decoder.Decoder
	nextDec   int
	mu        sync.Mutex
}

// NewServer creates a new server instance
func NewServer(outputDir, mode string, workers int) (*Server, error) {
	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	s := &Server{
		outputDir: outputDir,
		mode:      mode,
		workers:   workers,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024 * 1024,
			WriteBufferSize: 1024 * 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for local development
			},
		},
		decoders: make([]*decoder.Decoder, workers),
		nextDec:  0,
	}

	// Initialize decoders
	for i := 0; i < workers; i++ {
		s.decoders[i] = decoder.NewDecoder(mode)
	}

	log.Printf("Server initialized with %d workers, mode: %s, output: %s", workers, mode, outputDir)
	return s, nil
}

// StaticHandler serves static files from the web/server directory
func (s *Server) StaticHandler(fs http.FileSystem) http.Handler {
	return http.FileServer(fs)
}

// WSHandler handles WebSocket connections for video frame decoding
func (s *Server) WSHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("Client connected: %s", conn.RemoteAddr())

	// Get a decoder for this connection
	s.mu.Lock()
	dec := s.decoders[s.nextDec]
	s.nextDec = (s.nextDec + 1) % len(s.decoders)
	s.mu.Unlock()

	// Track decode state
	var currentFileID uint32 = 0

	for {
		// Read binary message
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		if msgType != websocket.BinaryMessage {
			// Skip non-binary messages
			continue
		}

		// Parse frame header: [format:1][mode:1][width:2][height:2][data:...]
		if len(data) < 6 {
			s.sendResponse(conn, DecodeResponse{Type: "error", Error: "invalid frame format"})
			continue
		}

		format := decoder.ImageFormat(data[0])
		_ = data[1] // mode (reserved for future)
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
			// Continue anyway, some formats may have padding
		}

		// Step 1: Scan, extract and decode
		result, extractedData, err := dec.ScanExtractDecode(pixelData, width, height, format)
		if err != nil {
			s.sendResponse(conn, DecodeResponse{Type: "error", Error: err.Error()})
			continue
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
			// No data extracted, send nodata response
			s.sendResponse(conn, DecodeResponse{
				Type:      "decode",
				Extracted: result.Extracted,
				NoData:    true,
			})
			continue
		}

		// Step 2: Fountain decode with extracted data
		fountainResult, err := dec.FountainDecode(extractedData)
		if err != nil {
			s.sendResponse(conn, DecodeResponse{Type: "error", Error: err.Error()})
			continue
		}

		// Check if file is complete
		if fountainResult.FileID > 0 {
			// File complete!
			newFileID := fountainResult.FileID
			if newFileID != currentFileID {
				// New file completed
				currentFileID = newFileID
				s.handleFileComplete(conn, newFileID)
			}
		}

		// Send decode response with progress
		s.sendResponse(conn, DecodeResponse{
			Type:      "decode",
			Bytes:     result.Bytes,
			Extracted: true,
			Progress:  fountainResult.Progress,
		})
	}

	log.Printf("Client disconnected: %s", conn.RemoteAddr())
}

// handleFileComplete handles a completed file decode
func (s *Server) handleFileComplete(conn *websocket.Conn, fileID uint32) {
	filename, err := decoder.GetFilename(fileID)
	if err != nil {
		s.sendResponse(conn, DecodeResponse{Type: "error", Error: fmt.Sprintf("get filename: %v", err)})
		return
	}

	if filename == "" {
		filename = fmt.Sprintf("cimbar_file_%d", fileID)
	}

	fileSize := decoder.GetFileSize(fileID)

	// Read decompressed data
	var allData []byte
	for {
		chunk, err := decoder.DecompressRead(fileID)
		if err != nil {
			log.Printf("DecompressRead error: %v", err)
			break
		}
		if len(chunk) == 0 {
			break
		}
		allData = append(allData, chunk...)
	}

	if len(allData) == 0 {
		s.sendResponse(conn, DecodeResponse{Type: "error", Error: "no data to write"})
		return
	}

	// Write to output directory
	outputPath := filepath.Join(s.outputDir, filename)
	if err := os.WriteFile(outputPath, allData, 0644); err != nil {
		s.sendResponse(conn, DecodeResponse{Type: "error", Error: fmt.Sprintf("write file: %v", err)})
		return
	}

	log.Printf("File saved: %s (%d bytes)", outputPath, len(allData))

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
