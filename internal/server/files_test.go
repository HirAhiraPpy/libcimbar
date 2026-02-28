package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/HirAhiraPpy/libcimbar/go/internal/server"
)

// TestFilesHandler_EmptyList tests getting file list for empty session
func TestFilesHandler_EmptyList(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.teardown()

	// Login
	loginReq := server.LoginRequest{Email: "files@example.com"}
	body, _ := json.Marshal(loginReq)

	loginResp, err := http.Post(ts.httpSrv.URL+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Login request failed: %v", err)
	}
	defer loginResp.Body.Close()

	var loginData server.LoginResponse
	json.NewDecoder(loginResp.Body).Decode(&loginData)

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

// TestDeleteFile_NotFound tests deleting a non-existent file
func TestDeleteFile_NotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.teardown()

	// Login
	loginReq := server.LoginRequest{Email: "deletenf@example.com"}
	body, _ := json.Marshal(loginReq)

	loginResp, err := http.Post(ts.httpSrv.URL+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Login request failed: %v", err)
	}
	defer loginResp.Body.Close()

	var loginData server.LoginResponse
	json.NewDecoder(loginResp.Body).Decode(&loginData)

	// Try to delete non-existent file
	deleteReq, _ := http.NewRequest("DELETE", ts.httpSrv.URL+"/api/files/nonexistent", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+loginData.SessionToken)

	deleteResp, err := http.DefaultClient.Do(deleteReq)
	if err != nil {
		t.Fatalf("Delete request failed: %v", err)
	}
	defer deleteResp.Body.Close()

	if deleteResp.StatusCode != http.StatusNotFound {
		t.Fatalf("Expected status 404, got %d", deleteResp.StatusCode)
	}
}

// TestDownloadFile_Unauthorized tests downloading without auth
func TestDownloadFile_Unauthorized(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.teardown()

	// Try to download without token
	downloadURL := ts.httpSrv.URL + "/api/download?hash=testhash"
	resp, err := http.Get(downloadURL)
	if err != nil {
		t.Fatalf("Download request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("Expected status 401, got %d", resp.StatusCode)
	}
}
