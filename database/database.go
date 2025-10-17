package database

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// CopyDatabase creates a copy of the source SQLite database at the destination path
func CopyDatabase(sourcePath string, destPath string) error {
	// Validate source database exists
	if _, err := os.Stat(sourcePath); err != nil {
		return fmt.Errorf("source database does not exist: %w", err)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Open source database
	sourceDB, err := sql.Open("sqlite3", sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer sourceDB.Close()

	// Test source database connectivity
	if err := sourceDB.Ping(); err != nil {
		return fmt.Errorf("failed to ping source database: %w", err)
	}

	// Close source database before copying to avoid locking issues
	sourceDB.Close()

	// Use SQLite backup API for reliable copying
	return copyDatabaseWithBackup(sourcePath, destPath)
}

// copyDatabaseWithBackup uses SQLite's backup API for reliable database copying
func copyDatabaseWithBackup(sourcePath string, destPath string) error {
	// Open source database
	sourceDB, err := sql.Open("sqlite3", sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer sourceDB.Close()

	// Open destination database (this will create it)
	destDB, err := sql.Open("sqlite3", destPath)
	if err != nil {
		return fmt.Errorf("failed to open destination database: %w", err)
	}
	defer destDB.Close()

	// Attach source database to destination
	attachQuery := fmt.Sprintf("ATTACH DATABASE '%s' AS source", sourcePath)
	if _, err := destDB.Exec(attachQuery); err != nil {
		return fmt.Errorf("failed to attach source database: %w", err)
	}
	defer destDB.Exec("DETACH DATABASE source")

	// Get list of tables to copy (excluding sqlite_sequence and sqlite_master)
	rows, err := destDB.Query("SELECT name, sql FROM source.sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return fmt.Errorf("failed to get table list: %w", err)
	}
	defer rows.Close()

	var tables []struct {
		name string
		sql  string
	}

	for rows.Next() {
		var name, sql string
		if err := rows.Scan(&name, &sql); err != nil {
			return fmt.Errorf("failed to scan table info: %w", err)
		}
		tables = append(tables, struct {
			name string
			sql  string
		}{name, sql})
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate tables: %w", err)
	}

	// Copy each table
	for _, table := range tables {
		// Create table in destination
		if _, err := destDB.Exec(table.sql); err != nil {
			return fmt.Errorf("failed to create table %s: %w", table.name, err)
		}

		// Copy data
		copyQuery := fmt.Sprintf("INSERT INTO main.%s SELECT * FROM source.%s", table.name, table.name)
		if _, err := destDB.Exec(copyQuery); err != nil {
			return fmt.Errorf("failed to copy data for table %s: %w", table.name, err)
		}
	}

	// Copy indexes
	indexRows, err := destDB.Query("SELECT name, sql FROM source.sqlite_master WHERE type='index' AND sql IS NOT NULL")
	if err != nil {
		return fmt.Errorf("failed to get index list: %w", err)
	}
	defer indexRows.Close()

	for indexRows.Next() {
		var name, sql string
		if err := indexRows.Scan(&name, &sql); err != nil {
			return fmt.Errorf("failed to scan index info: %w", err)
		}

		if _, err := destDB.Exec(sql); err != nil {
			return fmt.Errorf("failed to create index %s: %w", name, err)
		}
	}

	return nil
}

// CopyDatabaseFile performs a simple file copy of the database
// This is faster but less reliable than the backup method for active databases
func CopyDatabaseFile(sourcePath string, destPath string) error {
	// Validate source database exists
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("source database does not exist: %w", err)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Open source file
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source database file: %w", err)
	}
	defer sourceFile.Close()

	// Create destination file
	destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination database file: %w", err)
	}
	defer destFile.Close()

	// Copy the file
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy database file: %w", err)
	}

	return nil
}

// VerifyDatabaseIntegrity checks that the database is valid and accessible
func VerifyDatabaseIntegrity(dbPath string) error {
	// Open database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Test connectivity
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Run integrity check
	var integrityCheck string
	err = db.QueryRow("PRAGMA integrity_check").Scan(&integrityCheck)
	if err != nil {
		return fmt.Errorf("failed to run integrity check: %w", err)
	}

	if integrityCheck != "ok" {
		return fmt.Errorf("database integrity check failed: %s", integrityCheck)
	}

	// Check that we can query sqlite_master
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to query sqlite_master: %w", err)
	}

	if count == 0 {
		return fmt.Errorf("database contains no tables")
	}

	return nil
}

// GetDatabaseInfo returns information about the database
func GetDatabaseInfo(dbPath string) (map[string]interface{}, error) {
	info := make(map[string]interface{})

	// Get file info
	fileInfo, err := os.Stat(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat database file: %w", err)
	}

	info["size"] = fileInfo.Size()
	info["modified"] = fileInfo.ModTime()
	info["permissions"] = fileInfo.Mode()

	// Open database and get schema info
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return info, nil // Return file info even if we can't open the database
	}
	defer db.Close()

	// Get table count
	var tableCount int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&tableCount)
	if err == nil {
		info["table_count"] = tableCount
	}

	// Get table names
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err == nil {
		defer rows.Close()
		var tables []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err == nil {
				tables = append(tables, name)
			}
		}
		info["tables"] = tables
	}

	// Get page count and size
	var pageCount, pageSize int
	err = db.QueryRow("PRAGMA page_count").Scan(&pageCount)
	if err == nil {
		info["page_count"] = pageCount
	}

	err = db.QueryRow("PRAGMA page_size").Scan(&pageSize)
	if err == nil {
		info["page_size"] = pageSize
	}

	return info, nil
}

// CreateTempDatabaseCopy creates a temporary copy of the database
func CreateTempDatabaseCopy(sourcePath string, prefix string) (string, error) {
	// Create temporary file with laforge prefix for safety
	tempDir, err := os.MkdirTemp("", "laforge-worktree-state-")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	tempPath := filepath.Join(tempDir, "tasks.db")

	// Copy database to temporary location
	if err := CopyDatabase(sourcePath, tempPath); err != nil {
		os.Remove(tempPath) // Clean up on error
		return "", fmt.Errorf("failed to copy database to temporary location: %w", err)
	}

	return tempPath, nil
}

// CleanupTempDatabase removes a temporary database copy
func CleanupTempDatabase(tempPath string) error {
	if tempPath == "" {
		return nil
	}

	// Verify it's actually a temporary file before deleting
	if !isTempFile(tempPath) {
		return fmt.Errorf("refusing to delete non-temporary file: %s", tempPath)
	}

	return os.Remove(tempPath)
}

// isTempFile checks if a path appears to be a temporary file
func isTempFile(path string) bool {
	// Check if the filename contains typical temporary file patterns
	base := filepath.Base(path)
	return len(base) > 0 && base[0] == '.' ||
		contains(base, "-tmp-") ||
		contains(base, "-temp-") ||
		contains(base, ".tmp") ||
		contains(base, "_tmp_") ||
		contains(base, "laforge-")
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsAt(s, substr)
}

// containsAt checks if a string contains a substring at any position
func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
