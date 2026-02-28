package server

import (
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"time"
)

// LoginRequest represents a login request
type LoginRequest struct {
	Email string `json:"email"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	Success      bool   `json:"success"`
	SessionToken string `json:"session_token,omitempty"`
	Error        string `json:"error,omitempty"`
}

// loginAndSetCookie logs in user and sets cookie
func (s *Server) loginAndSetCookie(w http.ResponseWriter, email, token string) {
	// Set cookie (7 days expiry)
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		MaxAge:   604800, // 7 days
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ValidateSessionResponse represents a session validation response
type ValidateSessionResponse struct {
	Valid   bool   `json:"valid"`
	Email   string `json:"email,omitempty"`
	Error   string `json:"error,omitempty"`
}

// UserInfoResponse represents user information
type UserInfoResponse struct {
	Email       string `json:"email"`
	CreatedAt   string `json:"created_at,omitempty"`
	LastLoginAt string `json:"last_login_at,omitempty"`
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// LoginHandler handles user login
func (s *Server) LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "invalid request body"})
		return
	}

	// Validate email format
	if !emailRegex.MatchString(req.Email) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "invalid email format"})
		return
	}

	// Create session
	token, err := s.sessionManager.CreateSession(req.Email)
	if err != nil {
		log.Printf("Failed to create session for %s: %v", req.Email, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(LoginResponse{Success: false, Error: "failed to create session"})
		return
	}

	log.Printf("User logged in: %s", req.Email)

	w.Header().Set("Content-Type", "application/json")
	s.loginAndSetCookie(w, req.Email, token)
	json.NewEncoder(w).Encode(LoginResponse{
		Success:      true,
		SessionToken: token,
	})
}

// LogoutHandler handles user logout
func (s *Server) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := getSessionToken(r)
	if token == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "no session token"})
		return
	}

	if err := s.sessionManager.InvalidateSession(token); err != nil {
		log.Printf("Failed to invalidate session: %v", err)
	}

	log.Println("User logged out")

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"success": "true"})
}

// ValidateSessionHandler validates a session token
func (s *Server) ValidateSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := getSessionToken(r)
	if token == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ValidateSessionResponse{Valid: false, Error: "no session token"})
		return
	}

	sess, err := s.sessionManager.ValidateSession(token)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ValidateSessionResponse{Valid: false, Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ValidateSessionResponse{
		Valid: true,
		Email: sess.Email,
	})
}

// UserInfoHandler returns current user information
func (s *Server) UserInfoHandler(w http.ResponseWriter, r *http.Request) {
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

	lastLoginAt := ""
	if sess.UserLastLoginAt.Valid {
		lastLoginAt = sess.UserLastLoginAt.Time.Format(time.RFC3339)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserInfoResponse{
		Email:       sess.Email,
		CreatedAt:   sess.UserCreatedAt.Format(time.RFC3339),
		LastLoginAt: lastLoginAt,
	})
}

// getSessionToken extracts session token from request
func getSessionToken(r *http.Request) string {
	// Try Authorization header first
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" && len(authHeader) > 7 {
		return authHeader[7:] // Remove "Bearer " prefix
	}

	// Try cookie
	cookie, err := r.Cookie("session_token")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// Try URL query parameter
	return r.URL.Query().Get("token")
}
