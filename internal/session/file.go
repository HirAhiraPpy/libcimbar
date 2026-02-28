// Package session provides user session management with SQLite storage.
package session

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// File represents a decoded file in the system
type File struct {
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
}

// SaveFile saves file metadata to the database
func (d *Database) SaveFile(sessionID int64, fileHash, originalFilename, filePath string, fileSize int64) (int64, error) {
	result, err := d.db.Exec(
		`INSERT INTO files (session_id, file_hash, original_filename, file_size, file_path, is_complete)
		 VALUES (?, ?, ?, ?, ?, 0)
		 ON CONFLICT(session_id, file_hash) DO UPDATE SET
		 file_size = excluded.file_size,
		 file_path = excluded.file_path,
		 is_complete = 0`,
		sessionID, fileHash, originalFilename, fileSize, filePath,
	)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

// MarkFileComplete marks a file as complete
func (d *Database) MarkFileComplete(sessionID int64, fileHash string, fountainFileID uint32, fountainFileSize int64, blocksRequired int) error {
	_, err := d.db.Exec(
		`UPDATE files SET is_complete = 1, fountain_file_id = ?, fountain_file_size = ?, blocks_required = ?
		 WHERE session_id = ? AND file_hash = ?`,
		fountainFileID, fountainFileSize, blocksRequired, sessionID, fileHash,
	)
	return err
}

// GetFileByHash gets a file by session ID and file hash
func (d *Database) GetFileByHash(sessionID int64, fileHash string) (*File, error) {
	var f File
	var deletedAt sql.NullTime

	err := d.db.QueryRow(
		`SELECT id, session_id, file_hash, original_filename, file_size, file_path,
		        created_at, is_complete, deleted_at, fountain_file_id, fountain_file_size, blocks_required
		 FROM files
		 WHERE session_id = ? AND file_hash = ?`,
		sessionID, fileHash,
	).Scan(
		&f.ID, &f.SessionID, &f.FileHash, &f.OriginalFilename, &f.FileSize, &f.FilePath,
		&f.CreatedAt, &f.IsComplete, &deletedAt, &f.FountainFileID, &f.FountainFileSize, &f.BlocksRequired,
	)

	if err == sql.ErrNoRows {
		return nil, ErrFileNotFound
	}
	if err != nil {
		return nil, err
	}

	if deletedAt.Valid {
		f.DeletedAt = deletedAt.Time
	}

	return &f, nil
}

// GetSessionFiles gets all non-deleted files for a session
func (d *Database) GetSessionFiles(sessionID int64) ([]*File, error) {
	rows, err := d.db.Query(
		`SELECT id, session_id, file_hash, original_filename, file_size, file_path,
		        created_at, is_complete, fountain_file_id, fountain_file_size, blocks_required
		 FROM files
		 WHERE session_id = ? AND deleted_at IS NULL
		 ORDER BY created_at DESC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		var f File
		err := rows.Scan(
			&f.ID, &f.SessionID, &f.FileHash, &f.OriginalFilename, &f.FileSize, &f.FilePath,
			&f.CreatedAt, &f.IsComplete, &f.FountainFileID, &f.FountainFileSize, &f.BlocksRequired,
		)
		if err != nil {
			return nil, err
		}
		files = append(files, &f)
	}

	return files, rows.Err()
}

// SoftDeleteFile marks a file as deleted (soft delete)
func (d *Database) SoftDeleteFile(sessionID int64, fileHash string) error {
	_, err := d.db.Exec(
		"UPDATE files SET deleted_at = CURRENT_TIMESTAMP WHERE session_id = ? AND file_hash = ?",
		sessionID, fileHash,
	)
	return err
}

// DeleteAllSessionFiles marks all files for a session as deleted
func (d *Database) DeleteAllSessionFiles(sessionID int64) (int64, error) {
	result, err := d.db.Exec(
		"UPDATE files SET deleted_at = CURRENT_TIMESTAMP WHERE session_id = ? AND deleted_at IS NULL",
		sessionID,
	)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// AddReceivedBlock adds a received block to a file
func (d *Database) AddReceivedBlock(fileID int64, blockID int) error {
	_, err := d.db.Exec(
		"INSERT OR IGNORE INTO received_blocks (file_id, block_id) VALUES (?, ?)",
		fileID, blockID,
	)
	return err
}

// GetReceivedBlocks gets all received block IDs for a file
func (d *Database) GetReceivedBlocks(fileID int64) ([]int, error) {
	rows, err := d.db.Query(
		"SELECT block_id FROM received_blocks WHERE file_id = ? ORDER BY block_id",
		fileID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blocks []int
	for rows.Next() {
		var blockID int
		if err := rows.Scan(&blockID); err != nil {
			return nil, err
		}
		blocks = append(blocks, blockID)
	}

	return blocks, rows.Err()
}

// HasReceivedBlock checks if a block has been received
func (d *Database) HasReceivedBlock(fileID int64, blockID int) (bool, error) {
	var count int
	err := d.db.QueryRow(
		"SELECT COUNT(*) FROM received_blocks WHERE file_id = ? AND block_id = ?",
		fileID, blockID,
	).Scan(&count)

	return count > 0, err
}

// EnsureCacheDir creates the cache directory for a session
func EnsureCacheDir(cacheBaseDir string, sessionID int64) (string, error) {
	dir := filepath.Join(cacheBaseDir, fmt.Sprintf("%d", sessionID))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// ComputeFileHash computes SHA256 hash of a filename for unique identification
func ComputeFileHash(filename string) string {
	hash := sha256.Sum256([]byte(filename))
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes (16 hex chars)
}

// Error definition
var (
	ErrFileNotFound = fmt.Errorf("file not found")
)
