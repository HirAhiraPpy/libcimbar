package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/HirAhiraPpy/libcimbar/go/internal/server"
)

// testServer holds testing resources
type testServer struct {
	srv      *server.Server
	httpSrv  *httptest.Server
	dbPath   string
	cacheDir string
}

// setupTestServer creates a test server instance
func setupTestServer(t *testing.T) *testServer {
	t.Helper()

	// Create temp directory for test data
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	cacheDir := tmpDir + "/cache"
	outputDir := tmpDir + "/output"

	// Create server config
	cfg := server.ServerConfig{
		OutputDir: outputDir,
		CacheDir:  cacheDir,
		Mode:      "Auto",
		Workers:   2,
		DBPath:    dbPath,
	}

	srv, err := server.NewServer(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create test HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", srv.LoginHandler)
	mux.HandleFunc("/api/logout", srv.LogoutHandler)
	mux.HandleFunc("/api/session/validate", srv.ValidateSessionHandler)
	mux.HandleFunc("/api/user", srv.UserInfoHandler)
	mux.HandleFunc("/api/files", srv.FilesHandler)
	mux.HandleFunc("/api/files/", srv.FileDetailHandler)
	mux.HandleFunc("/api/download", srv.DownloadHandler)

	httpSrv := httptest.NewServer(mux)

	return &testServer{
		srv:      srv,
		httpSrv:  httpSrv,
		dbPath:   dbPath,
		cacheDir: cacheDir,
	}
}

// teardown closes all resources
func (ts *testServer) teardown() {
	ts.httpSrv.Close()
	ts.srv.Close()
}

// TestLogin_NewUser_AutoRegister tests that a new user is automatically registered
func TestLogin_NewUser_AutoRegister(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.teardown()

	loginReq := server.LoginRequest{Email: "test@example.com"}
	body, _ := json.Marshal(loginReq)

	resp, err := http.Post(ts.httpSrv.URL+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var loginResp server.LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !loginResp.Success {
		t.Fatalf("Expected success=true, got false")
	}

	if loginResp.SessionToken == "" {
		t.Fatalf("Expected session_token, got empty string")
	}

	if len(loginResp.SessionToken) != 64 {
		t.Fatalf("Expected 64-char token, got %d", len(loginResp.SessionToken))
	}
}

// TestLogin_InvalidEmail_ReturnsError tests email validation
func TestLogin_InvalidEmail_ReturnsError(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.teardown()

	loginReq := server.LoginRequest{Email: "invalid-email"}
	body, _ := json.Marshal(loginReq)

	resp, err := http.Post(ts.httpSrv.URL+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", resp.StatusCode)
	}

	var loginResp server.LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if loginResp.Success {
		t.Fatalf("Expected success=false, got true")
	}

	if loginResp.Error == "" {
		t.Fatalf("Expected error message, got empty")
	}
}

// TestSessionValidate_ValidToken tests session validation with valid token
func TestSessionValidate_ValidToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.teardown()

	// First login to get token
	loginReq := server.LoginRequest{Email: "validate@example.com"}
	body, _ := json.Marshal(loginReq)

	loginResp, err := http.Post(ts.httpSrv.URL+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Login request failed: %v", err)
	}
	defer loginResp.Body.Close()

	var loginData server.LoginResponse
	json.NewDecoder(loginResp.Body).Decode(&loginData)

	// Validate the token using Authorization header (cookie also works in browser)
	req, _ := http.NewRequest("GET", ts.httpSrv.URL+"/api/session/validate", nil)
	req.Header.Set("Authorization", "Bearer "+loginData.SessionToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Validate request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var validateResp server.ValidateSessionResponse
	json.NewDecoder(resp.Body).Decode(&validateResp)

	if !validateResp.Valid {
		t.Fatalf("Expected valid=true, got false")
	}

	if validateResp.Email != "validate@example.com" {
		t.Fatalf("Expected email 'validate@example.com', got '%s'", validateResp.Email)
	}
}

// TestSessionValidate_InvalidToken tests session validation with invalid token
func TestSessionValidate_InvalidToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.teardown()

	req, _ := http.NewRequest("GET", ts.httpSrv.URL+"/api/session/validate", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-12345")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Validate request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected status 401, got %d", resp.StatusCode)
	}

	var validateResp server.ValidateSessionResponse
	json.NewDecoder(resp.Body).Decode(&validateResp)

	if validateResp.Valid {
		t.Fatalf("Expected valid=false, got true")
	}
}

// TestLogout_InvalidatesSession tests that logout invalidates the session
func TestLogout_InvalidatesSession(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.teardown()

	// Login
	loginReq := server.LoginRequest{Email: "logout@example.com"}
	body, _ := json.Marshal(loginReq)

	loginResp, _ := http.Post(ts.httpSrv.URL+"/api/login", "application/json", bytes.NewReader(body))
	var loginData server.LoginResponse
	json.NewDecoder(loginResp.Body).Decode(&loginData)
	loginResp.Body.Close()

	token := loginData.SessionToken

	// Logout
	req, _ := http.NewRequest("POST", ts.httpSrv.URL+"/api/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Logout request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Try to validate the token after logout
	req2, _ := http.NewRequest("GET", ts.httpSrv.URL+"/api/session/validate", nil)
	req2.Header.Set("Authorization", "Bearer "+token)

	resp2, _ := http.DefaultClient.Do(req2)
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected status 401 after logout, got %d", resp2.StatusCode)
	}
}

// TestUserInfo tests getting user information
func TestUserInfo(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.teardown()

	// Login
	loginReq := server.LoginRequest{Email: "userinfo@example.com"}
	body, _ := json.Marshal(loginReq)

	loginResp, _ := http.Post(ts.httpSrv.URL+"/api/login", "application/json", bytes.NewReader(body))
	var loginData server.LoginResponse
	json.NewDecoder(loginResp.Body).Decode(&loginData)
	loginResp.Body.Close()

	// Get user info
	req, _ := http.NewRequest("GET", ts.httpSrv.URL+"/api/user", nil)
	req.Header.Set("Authorization", "Bearer "+loginData.SessionToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("User info request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var userInfo server.UserInfoResponse
	json.NewDecoder(resp.Body).Decode(&userInfo)

	if userInfo.Email != "userinfo@example.com" {
		t.Fatalf("Expected email 'userinfo@example.com', got '%s'", userInfo.Email)
	}
}

// TestFileIsolation_DifferentUsers tests that users cannot access each other's files
func TestFileIsolation_DifferentUsers(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.teardown()

	// Login user 1
	loginReq1 := server.LoginRequest{Email: "user1@example.com"}
	body1, _ := json.Marshal(loginReq1)
	loginResp1, _ := http.Post(ts.httpSrv.URL+"/api/login", "application/json", bytes.NewReader(body1))
	var loginData1 server.LoginResponse
	json.NewDecoder(loginResp1.Body).Decode(&loginData1)
	loginResp1.Body.Close()

	// Login user 2
	loginReq2 := server.LoginRequest{Email: "user2@example.com"}
	body2, _ := json.Marshal(loginReq2)
	loginResp2, _ := http.Post(ts.httpSrv.URL+"/api/login", "application/json", bytes.NewReader(body2))
	var loginData2 server.LoginResponse
	json.NewDecoder(loginResp2.Body).Decode(&loginData2)
	loginResp2.Body.Close()

	// User 1 gets files (should be empty)
	req1, _ := http.NewRequest("GET", ts.httpSrv.URL+"/api/files", nil)
	req1.Header.Set("Authorization", "Bearer "+loginData1.SessionToken)

	resp1, _ := http.DefaultClient.Do(req1)
	defer resp1.Body.Close()

	var filesResp1 server.FilesListResponse
	json.NewDecoder(resp1.Body).Decode(&filesResp1)

	// User 2 gets files (should be empty)
	req2, _ := http.NewRequest("GET", ts.httpSrv.URL+"/api/files", nil)
	req2.Header.Set("Authorization", "Bearer "+loginData2.SessionToken)

	resp2, _ := http.DefaultClient.Do(req2)
	defer resp2.Body.Close()

	var filesResp2 server.FilesListResponse
	json.NewDecoder(resp2.Body).Decode(&filesResp2)

	// Both should have empty file lists
	if len(filesResp1.Files) != 0 {
		t.Fatalf("User 1 should have 0 files, got %d", len(filesResp1.Files))
	}
	if len(filesResp2.Files) != 0 {
		t.Fatalf("User 2 should have 0 files, got %d", len(filesResp2.Files))
	}
}

// TestGetFiles_EmptySession tests getting file list for empty session
func TestGetFiles_EmptySession(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.teardown()

	// Login
	loginReq := server.LoginRequest{Email: "empty@example.com"}
	body, _ := json.Marshal(loginReq)
	loginResp, _ := http.Post(ts.httpSrv.URL+"/api/login", "application/json", bytes.NewReader(body))
	var loginData server.LoginResponse
	json.NewDecoder(loginResp.Body).Decode(&loginData)
	loginResp.Body.Close()

	// Get files
	req, _ := http.NewRequest("GET", ts.httpSrv.URL+"/api/files", nil)
	req.Header.Set("Authorization", "Bearer "+loginData.SessionToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Get files request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var filesResp server.FilesListResponse
	json.NewDecoder(resp.Body).Decode(&filesResp)

	if filesResp.Files == nil {
		t.Fatalf("Expected files array, got nil")
	}

	if len(filesResp.Files) != 0 {
		t.Fatalf("Expected 0 files, got %d", len(filesResp.Files))
	}
}

// TestUnauthenticatedAccess tests accessing protected endpoints without auth
func TestUnauthenticatedAccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.teardown()

	// Try to get files without token
	resp, err := http.Get(ts.httpSrv.URL + "/api/files")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected status 401, got %d", resp.StatusCode)
	}

	// Try to get user info without token
	resp2, err := http.Get(ts.httpSrv.URL + "/api/user")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected status 401, got %d", resp2.StatusCode)
	}
}

// TestMain runs all tests
func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
