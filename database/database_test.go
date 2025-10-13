package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestCopyDatabase(t *testing.T) {
	// Create a temporary directory for test databases
	tempDir, err := os.MkdirTemp("", "laforge-db-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourcePath := filepath.Join(tempDir, "source.db")
	destPath := filepath.Join(tempDir, "dest.db")

	// Create a test database
	sourceDB, err := createTestDatabase(sourcePath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	sourceDB.Close()

	// Copy the database
	if err := CopyDatabase(sourcePath, destPath); err != nil {
		t.Fatalf("Failed to copy database: %v", err)
	}

	// Verify the copy exists
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("Destination database does not exist: %v", err)
	}

	// Verify the copy is valid
	if err := VerifyDatabaseIntegrity(destPath); err != nil {
		t.Errorf("Destination database integrity check failed: %v", err)
	}

	// Verify data was copied
	destDB, err := sql.Open("sqlite3", destPath)
	if err != nil {
		t.Fatalf("Failed to open destination database: %v", err)
	}
	defer destDB.Close()

	var count int
	err = destDB.QueryRow("SELECT COUNT(*) FROM test_table").Scan(&count)
	if err != nil {
		t.Errorf("Failed to query destination database: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 rows in destination database, got %d", count)
	}
}

func TestCopyDatabaseFile(t *testing.T) {
	// Create a temporary directory for test databases
	tempDir, err := os.MkdirTemp("", "laforge-db-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourcePath := filepath.Join(tempDir, "source.db")
	destPath := filepath.Join(tempDir, "dest.db")

	// Create a test database
	sourceDB, err := createTestDatabase(sourcePath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	sourceDB.Close()

	// Copy the database file
	if err := CopyDatabaseFile(sourcePath, destPath); err != nil {
		t.Fatalf("Failed to copy database file: %v", err)
	}

	// Verify the copy exists and has same size
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatalf("Failed to stat source database: %v", err)
	}

	destInfo, err := os.Stat(destPath)
	if err != nil {
		t.Errorf("Failed to stat destination database: %v", err)
	}

	if destInfo.Size() != sourceInfo.Size() {
		t.Errorf("Destination database size (%d) differs from source (%d)", destInfo.Size(), sourceInfo.Size())
	}

	// Verify the copy is valid
	if err := VerifyDatabaseIntegrity(destPath); err != nil {
		t.Errorf("Destination database integrity check failed: %v", err)
	}
}

func TestVerifyDatabaseIntegrity(t *testing.T) {
	// Create a temporary directory for test databases
	tempDir, err := os.MkdirTemp("", "laforge-db-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// Test with non-existent database
	if err := VerifyDatabaseIntegrity(dbPath); err == nil {
		t.Error("Expected error for non-existent database")
	}

	// Create a valid database
	db, err := createTestDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	db.Close()

	// Test with valid database
	if err := VerifyDatabaseIntegrity(dbPath); err != nil {
		t.Errorf("Database integrity check failed for valid database: %v", err)
	}

	// Test with corrupted database (empty file)
	corruptedPath := filepath.Join(tempDir, "corrupted.db")
	if err := os.WriteFile(corruptedPath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create corrupted database file: %v", err)
	}

	if err := VerifyDatabaseIntegrity(corruptedPath); err == nil {
		t.Error("Expected error for corrupted database")
	}
}

func TestGetDatabaseInfo(t *testing.T) {
	// Create a temporary directory for test databases
	tempDir, err := os.MkdirTemp("", "laforge-db-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// Create a test database
	db, err := createTestDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	db.Close()

	// Get database info
	info, err := GetDatabaseInfo(dbPath)
	if err != nil {
		t.Fatalf("Failed to get database info: %v", err)
	}

	// Verify expected fields exist
	requiredFields := []string{"size", "modified", "permissions", "table_count", "tables"}
	for _, field := range requiredFields {
		if _, ok := info[field]; !ok {
			t.Errorf("Expected field '%s' not found in database info", field)
		}
	}

	// Verify table count
	if tableCount, ok := info["table_count"].(int); !ok || tableCount < 1 {
		t.Errorf("Expected at least 1 table, got %v", info["table_count"])
	}

	// Verify tables list contains our test table
	tables, ok := info["tables"].([]string)
	if !ok {
		t.Errorf("Expected tables to be []string, got %T", info["tables"])
	} else {
		found := false
		for _, table := range tables {
			if table == "test_table" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Test table 'test_table' not found in tables list")
		}
	}
}

func TestCreateTempDatabaseCopy(t *testing.T) {
	// Create a temporary directory for test databases
	tempDir, err := os.MkdirTemp("", "laforge-db-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourcePath := filepath.Join(tempDir, "source.db")

	// Create a test database
	sourceDB, err := createTestDatabase(sourcePath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	sourceDB.Close()

	// Create temporary copy
	tempPath, err := CreateTempDatabaseCopy(sourcePath, "test")
	if err != nil {
		t.Fatalf("Failed to create temporary database copy: %v", err)
	}
	defer CleanupTempDatabase(tempPath)

	// Verify the temporary copy exists
	if _, err := os.Stat(tempPath); err != nil {
		t.Errorf("Temporary database copy does not exist: %v", err)
	}

	// Verify it's a valid database
	if err := VerifyDatabaseIntegrity(tempPath); err != nil {
		t.Errorf("Temporary database copy integrity check failed: %v", err)
	}

	// Verify it has the expected prefix pattern
	if !isTempFile(tempPath) {
		t.Errorf("Temporary database path does not have expected temp pattern: %s", tempPath)
	}
}

func TestCleanupTempDatabase(t *testing.T) {
	// Create a temporary directory for test databases
	tempDir, err := os.MkdirTemp("", "laforge-db-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	sourcePath := filepath.Join(tempDir, "source.db")

	// Create a test database
	sourceDB, err := createTestDatabase(sourcePath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	sourceDB.Close()

	// Create temporary copy
	tempPath, err := CreateTempDatabaseCopy(sourcePath, "test")
	if err != nil {
		t.Fatalf("Failed to create temporary database copy: %v", err)
	}

	// Verify it exists
	if _, err := os.Stat(tempPath); err != nil {
		t.Fatalf("Temporary database copy does not exist: %v", err)
	}

	// Clean it up
	if err := CleanupTempDatabase(tempPath); err != nil {
		t.Fatalf("Failed to cleanup temporary database: %v", err)
	}

	// Verify it no longer exists
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Errorf("Temporary database copy still exists after cleanup")
	}

	// Test cleanup of non-temporary file (should fail)
	if err := CleanupTempDatabase(sourcePath); err == nil {
		t.Error("Expected error when trying to cleanup non-temporary file")
	}
}

// createTestDatabase creates a simple test database with some data
func createTestDatabase(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create a test table
	_, err = db.Exec(`
		CREATE TABLE test_table (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			value INTEGER
		)`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Insert some test data
	_, err = db.Exec("INSERT INTO test_table (name, value) VALUES ('test1', 1), ('test2', 2)")
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
