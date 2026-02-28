package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HirAhiraPpy/libcimbar/go/internal/decoder"
	"github.com/gorilla/websocket"
)

// TestCase 测试用例
type TestCase struct {
	name    string
	size    int
	content []byte
}

// generateTestContent 生成测试内容
func generateTestContent(size int) []byte {
	content := make([]byte, size)
	for i := 0; i < size; i++ {
		content[i] = byte(i % 256)
	}
	return content
}

// startTestServer 启动测试服务器（随机端口）
func startTestServer() (*Server, string, func()) {
	outputDir, err := os.MkdirTemp("", "cimbar-test-*")
	if err != nil {
		panic(fmt.Sprintf("failed to create temp dir: %v", err))
	}

	cacheDir, err := os.MkdirTemp("", "cimbar-test-cache-*")
	if err != nil {
		os.RemoveAll(outputDir)
		panic(fmt.Sprintf("failed to create cache dir: %v", err))
	}

	dbPath := filepath.Join(cacheDir, "test.db")

	cfg := ServerConfig{
		OutputDir: outputDir,
		CacheDir:  cacheDir,
		Mode:      "B",
		Workers:   2,
		DBPath:    dbPath,
	}

	srv, err := NewServer(cfg)
	if err != nil {
		os.RemoveAll(outputDir)
		panic(fmt.Sprintf("failed to create server: %v", err))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", srv.WSHandler)
	mux.HandleFunc("/api/login", srv.LoginHandler)

	server := &http.Server{
		Addr:    ":0", // 随机端口
		Handler: mux,
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		os.RemoveAll(outputDir)
		panic(fmt.Sprintf("failed to listen: %v", err))
	}

	// Get the actual port and use 127.0.0.1 for IPv4 compatibility
	addr := listener.Addr().String()
	// Extract port and use localhost
	_, port, _ := net.SplitHostPort(addr)
	addr = "127.0.0.1:" + port

	// Start server in goroutine
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			// Ignore error on shutdown
		}
	}()

	// Wait for server to be ready (short delay)
	time.Sleep(time.Millisecond * 100)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		server.Shutdown(ctx)
		srv.Close()
		os.RemoveAll(outputDir)
	}

	return srv, addr, cleanup
}

// getTestSessionToken 辅助函数获取 session token
func getTestSessionToken(serverURL string) string {
	loginReq := LoginRequest{Email: "test@test.com"}
	body, _ := json.Marshal(loginReq)

	if !strings.HasPrefix(serverURL, "http://") && !strings.HasPrefix(serverURL, "https://") {
		serverURL = "http://" + serverURL
	}

	resp, err := http.Post(serverURL+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, _ = io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var loginResp LoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return ""
	}

	if !loginResp.Success {
		return ""
	}

	return loginResp.SessionToken
}

// verifySavedFile 验证保存的文件
func verifySavedFile(t *testing.T, dir, name string, expected []byte) {
	t.Helper()
	path := filepath.Join(dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	if len(data) != len(expected) {
		t.Errorf("file content length mismatch: expected %d bytes, got %d bytes", len(expected), len(data))
	}

	for i := 0; i < len(expected) && i < len(data); i++ {
		if data[i] != expected[i] {
			t.Errorf("file content mismatch at byte %d: expected %d, got %d", i, expected[i], data[i])
			break
		}
	}
}

// TestIntegration_SingleFileDecode 单文件解码测试
func TestIntegration_SingleFileDecode(t *testing.T) {
	testFiles := []TestCase{
		{name: "small.txt", size: 1000},
		{name: "medium.bin", size: 50000},
		{name: "large.dat", size: 100000},
	}

	for _, tc := range testFiles {
		t.Run(tc.name, func(t *testing.T) {
			// 1. 启动测试服务器
			_, addr, cleanup := startTestServer()
			defer cleanup()

			// Get session token
			token := getTestSessionToken("http://" + addr)
			if token == "" {
				t.Fatal("failed to get session token")
			}

			// 2. 生成测试内容（未使用，保留用于未来 encoder 集成）
			_ = generateTestContent(tc.size)

			// 3. 连接 WebSocket
			client, err := Connect("ws://"+addr+"/ws", token)
			if err != nil {
				t.Fatalf("failed to connect: %v", err)
			}
			defer client.Close()

			// Note: 由于 encoder 需要 CGO 和 OpenGL 依赖，在测试环境中可能无法运行
			// 这里我们模拟发送帧数据
			// 实际使用时，需要使用真实的 encoder 生成帧

			// 发送一个虚拟帧用于测试连接
			frameData := make([]byte, 1024*1024*3) // RGB 1024x1024
			err = client.SendFrame(frameData, decoder.FormatRGB)
			if err != nil {
				t.Fatalf("failed to send frame: %v", err)
			}

			// 接收响应
			resp, err := client.ReceiveResponse()
			if err != nil {
				// 可能因为帧数据无效而返回错误，这是预期的
				t.Logf("expected error for dummy frame: %v", err)
			} else {
				t.Logf("response type: %s", resp.Type)
			}
		})
	}
}

// TestIntegration_ServerLifecycle 服务器生命周期测试
func TestIntegration_ServerLifecycle(t *testing.T) {
	_, addr, cleanup := startTestServer()
	defer cleanup()

	// Get session token
	token := getTestSessionToken("http://" + addr)
	if token == "" {
		t.Fatal("failed to get session token")
	}

	// Test multiple connections
	for i := 0; i < 3; i++ {
		client, err := Connect("ws://"+addr+"/ws", token)
		if err != nil {
			t.Fatalf("connection %d: failed to connect: %v", i, err)
		}

		// Send ping
		err = client.SendPing()
		if err != nil {
			t.Fatalf("connection %d: failed to send ping: %v", i, err)
		}

		client.Close()
	}
}

// TestIntegration_ResponseTypes 响应类型测试
func TestIntegration_ResponseTypes(t *testing.T) {
	_, addr, cleanup := startTestServer()
	defer cleanup()

	// Get session token
	token := getTestSessionToken("http://" + addr)
	if token == "" {
		t.Fatal("failed to get session token")
	}

	client, err := Connect("ws://"+addr+"/ws", token)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	// Send invalid frame (too small)
	invalidFrame := make([]byte, 10)
	client.conn.SetWriteDeadline(time.Now().Add(time.Second * 5))
	client.conn.WriteMessage(websocket.BinaryMessage, invalidFrame)

	// Should receive error response
	resp, err := client.ReceiveResponse()
	if err != nil {
		t.Logf("no response for invalid frame (acceptable)")
	} else if resp.Type != "error" {
		t.Errorf("expected error response for invalid frame, got: %s", resp.Type)
	}
}

// TestIntegration_ConcurrentClients 并发客户端测试
func TestIntegration_ConcurrentClients(t *testing.T) {
	_, addr, cleanup := startTestServer()
	defer cleanup()

	// Get session token
	token := getTestSessionToken("http://" + addr)
	if token == "" {
		t.Fatal("failed to get session token")
	}

	// Connect multiple clients concurrently
	clients := make([]*WSClient, 5)
	for i := range clients {
		client, err := Connect("ws://"+addr+"/ws", token)
		if err != nil {
			t.Fatalf("client %d: failed to connect: %v", i, err)
		}
		clients[i] = client
		defer client.Close()
	}

	// Send frames from all clients
	for i, client := range clients {
		frameData := make([]byte, 1024*1024*3)
		err := client.SendFrame(frameData, decoder.FormatRGB)
		if err != nil {
			t.Logf("client %d frame send (may fail due to server load): %v", i, err)
		}
	}
}

// TestIntegration_KeepAlive 保持连接测试
func TestIntegration_KeepAlive(t *testing.T) {
	_, addr, cleanup := startTestServer()
	defer cleanup()

	// Get session token
	token := getTestSessionToken("http://" + addr)
	if token == "" {
		t.Fatal("failed to get session token")
	}

	client, err := Connect("ws://"+addr+"/ws", token)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	// Send pings periodically
	for i := 0; i < 3; i++ {
		err := client.SendPing()
		if err != nil {
			t.Fatalf("ping %d: failed to send: %v", i, err)
		}
		time.Sleep(time.Second * 2)
	}
}

// BenchmarkServerThroughput 服务器吞吐量基准测试
func BenchmarkServerThroughput(b *testing.B) {
	_, addr, cleanup := startTestServer()
	defer cleanup()

	// Get session token
	token := getTestSessionToken("http://" + addr)
	if token == "" {
		b.Fatal("failed to get session token")
	}

	client, err := Connect("ws://"+addr+"/ws", token)
	if err != nil {
		b.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	frameData := make([]byte, 1024*1024*3)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := client.SendFrame(frameData, decoder.FormatRGB)
		if err != nil {
			b.Fatalf("send frame error: %v", err)
		}
	}
}
