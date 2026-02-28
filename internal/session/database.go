// Package session provides user session management with SQLite storage.
package session

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database wraps SQLite connection for session management
type Database struct {
	db *sql.DB
}

// NewDatabase creates a new database connection
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	d := &Database{db: db}
	if err := d.init(); err != nil {
		db.Close()
		return nil, err
	}

	return d, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// GetDB returns the underlying sql.DB connection
func (d *Database) GetDB() *sql.DB {
	return d.db
}

// init creates database tables if they don't exist
func (d *Database) init() error {
	schema := `
	-- 用户表
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT UNIQUE NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_login_at DATETIME
	);

	-- Session 表
	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER UNIQUE NOT NULL,
		session_token TEXT UNIQUE NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_active_at DATETIME,
		is_active BOOLEAN DEFAULT 1,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	-- 文件表（存储原始文件名与 hash 的映射）
	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL,
		file_hash TEXT NOT NULL,
		original_filename TEXT NOT NULL,
		file_size INTEGER NOT NULL,
		file_path TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		is_complete BOOLEAN DEFAULT 0,
		deleted_at DATETIME,
		-- 断点续传所需字段
		fountain_file_id INTEGER,
		fountain_file_size INTEGER,
		blocks_required INTEGER,
		UNIQUE(session_id, file_hash),
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
	);

	-- 已接收块表（用于断点续传）
	CREATE TABLE IF NOT EXISTS received_blocks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file_id INTEGER NOT NULL,
		block_id INTEGER NOT NULL,
		received_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(file_id, block_id),
		FOREIGN KEY (file_id) REFERENCES files(id) ON DELETE CASCADE
	);

	-- 索引
	CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(session_token);
	CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
	CREATE INDEX IF NOT EXISTS idx_files_session ON files(session_id);
	CREATE INDEX IF NOT EXISTS idx_files_hash ON files(file_hash);
	CREATE INDEX IF NOT EXISTS idx_files_deleted ON files(deleted_at);
	CREATE INDEX IF NOT EXISTS idx_blocks_file ON received_blocks(file_id);
	`

	_, err := d.db.Exec(schema)
	return err
}

// GetOrCreateUser gets existing user or creates new one
func (d *Database) GetOrCreateUser(email string) (int64, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Try to get existing user
	var userID int64
	err = tx.QueryRow("SELECT id FROM users WHERE email = ?", email).Scan(&userID)
	if err == nil {
		// User exists, update last_login_at
		_, err = tx.Exec("UPDATE users SET last_login_at = CURRENT_TIMESTAMP WHERE id = ?", userID)
		if err != nil {
			return 0, err
		}
		tx.Commit()
		return userID, nil
	}

	if err != sql.ErrNoRows {
		return 0, err
	}

	// Create new user
	result, err := tx.Exec("INSERT INTO users (email) VALUES (?)", email)
	if err != nil {
		return 0, err
	}

	userID, err = result.LastInsertId()
	if err != nil {
		return 0, err
	}

	tx.Commit()
	return userID, nil
}

// CreateSession creates a new session for a user, invalidating any existing session
func (d *Database) CreateSession(userID int64) (string, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	// Invalidate existing session for this user
	_, err = tx.Exec("UPDATE sessions SET is_active = 0 WHERE user_id = ?", userID)
	if err != nil {
		return "", err
	}

	// Generate session token
	token := generateSessionToken()

	// Create new session
	result, err := tx.Exec(
		"INSERT INTO sessions (user_id, session_token, last_active_at) VALUES (?, ?, CURRENT_TIMESTAMP)",
		userID, token,
	)
	if err != nil {
		return "", err
	}

	_, err = result.LastInsertId()
	if err != nil {
		return "", err
	}

	tx.Commit()
	return token, nil
}

// ValidateSession checks if a session token is valid and active
func (d *Database) ValidateSession(token string) (*Session, error) {
	var s Session
	err := d.db.QueryRow(
		`SELECT s.id, s.user_id, s.session_token, s.created_at, s.last_active_at, s.is_active,
		        u.email, u.created_at, u.last_login_at
		 FROM sessions s
		 JOIN users u ON s.user_id = u.id
		 WHERE s.session_token = ?`,
		token,
	).Scan(
		&s.ID, &s.UserID, &s.SessionToken, &s.CreatedAt, &s.LastActiveAt, &s.IsActive,
		&s.Email, &s.UserCreatedAt, &s.UserLastLoginAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrInvalidSession
	}
	if err != nil {
		return nil, err
	}

	if !s.IsActive {
		return nil, ErrInactiveSession
	}

	// Update last_active_at
	_, err = d.db.Exec("UPDATE sessions SET last_active_at = CURRENT_TIMESTAMP WHERE id = ?", s.ID)
	if err != nil {
		return nil, err
	}

	return &s, nil
}

// InvalidateSession marks a session as inactive
func (d *Database) InvalidateSession(token string) error {
	_, err := d.db.Exec("UPDATE sessions SET is_active = 0 WHERE session_token = ?", token)
	return err
}

// InvalidateUserSessions marks all sessions for a user as inactive
func (d *Database) InvalidateUserSessions(userID int64) error {
	_, err := d.db.Exec("UPDATE sessions SET is_active = 0 WHERE user_id = ?", userID)
	return err
}

// Session represents a user session with user info
type Session struct {
	ID             int64
	UserID         int64
	SessionToken   string
	CreatedAt      time.Time
	LastActiveAt   time.Time
	IsActive       bool
	// User info
	Email           string
	UserCreatedAt   time.Time
	UserLastLoginAt sql.NullTime
}

// Error definitions
var (
	ErrInvalidSession   = fmt.Errorf("invalid session")
	ErrInactiveSession  = fmt.Errorf("session is inactive")
	ErrSessionExpired   = fmt.Errorf("session expired")
)
