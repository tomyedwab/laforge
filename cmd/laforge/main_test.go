package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tomyedwab/laforge/database"
)

func TestCreateTaskDatabaseBackup(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "laforge-backup-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test database
	sourcePath := filepath.Join(tempDir, "tasks.db")
	if err := createTestDatabase(sourcePath); err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Test creating backup with step ID
	stepID := "S1"
	backupPath, err := createTaskDatabaseBackup(sourcePath, stepID)
	if err != nil {
		t.Fatalf("Failed to create task database backup: %v", err)
	}

	// Verify backup file exists
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf("Backup file does not exist: %v", err)
	}

	// Verify backup filename contains step ID
	expectedFilename := "tasks-S1.db"
	if filepath.Base(backupPath) != expectedFilename {
		t.Errorf("Expected backup filename %s, got %s", expectedFilename, filepath.Base(backupPath))
	}

	// Verify backup is in the same directory as source
	if filepath.Dir(backupPath) != filepath.Dir(sourcePath) {
		t.Errorf("Backup should be in same directory as source")
	}

	// Verify backup is a valid database
	if err := database.VerifyDatabaseIntegrity(backupPath); err != nil {
		t.Errorf("Backup database integrity check failed: %v", err)
	}
}

// createTestDatabase creates a simple test database with some data
func createTestDatabase(dbPath string) error {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Create a test table
	_, err = db.Exec(`
		CREATE TABLE test_table (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			value INTEGER
		)
	`)
	if err != nil {
		return err
	}

	// Insert some test data
	_, err = db.Exec("INSERT INTO test_table (name, value) VALUES ('test1', 100), ('test2', 200)")
	if err != nil {
		return err
	}

	return nil
}
