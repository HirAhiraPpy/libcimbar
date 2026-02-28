// Package session provides user session management with SQLite storage.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"

	"github.com/gorilla/websocket"
)

// SessionManager manages active user sessions and WebSocket connections
type SessionManager struct {
	db          *Database
	activeConns map[int64]*websocket.Conn // userID -> conn
	mu          sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager(db *Database) *SessionManager {
	return &SessionManager{
		db:          db,
		activeConns: make(map[int64]*websocket.Conn),
	}
}

// GetDatabase returns the underlying database
func (m *SessionManager) GetDatabase() *Database {
	return m.db
}

// CreateSession creates a new session for a user
func (m *SessionManager) CreateSession(email string) (string, error) {
	userID, err := m.db.GetOrCreateUser(email)
	if err != nil {
		return "", err
	}

	token, err := m.db.CreateSession(userID)
	if err != nil {
		return "", err
	}

	return token, nil
}

// ValidateSession validates a session token
func (m *SessionManager) ValidateSession(token string) (*Session, error) {
	return m.db.ValidateSession(token)
}

// InvalidateSession invalidates a session
func (m *SessionManager) InvalidateSession(token string) error {
	return m.db.InvalidateSession(token)
}

// RegisterConnection registers a WebSocket connection for a user
// Returns the previous connection if it exists (for closing)
func (m *SessionManager) RegisterConnection(userID int64, conn *websocket.Conn) *websocket.Conn {
	m.mu.Lock()
	defer m.mu.Unlock()

	prevConn := m.activeConns[userID]
	m.activeConns[userID] = conn
	return prevConn
}

// UnregisterConnection removes a WebSocket connection for a user
func (m *SessionManager) UnregisterConnection(userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.activeConns, userID)
}

// GetConnection gets the active WebSocket connection for a user
func (m *SessionManager) GetConnection(userID int64) *websocket.Conn {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.activeConns[userID]
}

// KickUser closes the active connection for a user and invalidates their session
func (m *SessionManager) KickUser(userID int64) error {
	m.mu.Lock()
	conn := m.activeConns[userID]
	delete(m.activeConns, userID)
	m.mu.Unlock()

	if conn != nil {
		// Send close message to client
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "logged in elsewhere"))
		conn.Close()
	}

	return m.db.InvalidateUserSessions(userID)
}

// generateSessionToken generates a random session token
func generateSessionToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}
