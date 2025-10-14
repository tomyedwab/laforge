package integration

import (
	"database/sql"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestProjectDirectoryValidation tests project directory structure and validation
func TestProjectDirectoryValidation(t *testing.T) {
	// Build the laforge binary for testing
	if err := exec.Command("go", "build", "-o", "laforge-test", "../../cmd/laforge").Run(); err != nil {
		t.Skip("Cannot build laforge binary")
	}
	defer os.Remove("laforge-test")

	projectID := "validation-test-project"
	cleanupTestProject(projectID)
	defer cleanupTestProject(projectID)

	// Initialize project
	cmd := exec.Command("./laforge-test", "init", projectID, "--name", "Validation Test Project")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to initialize project: %v\nOutput: %s", err, string(output))
	}

	// Test 1: Validate project configuration file structure
	t.Run("Project configuration file", func(t *testing.T) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("Failed to get home directory: %v", err)
		}
		projectDir := filepath.Join(homeDir, ".laforge", "projects", projectID)
		configPath := filepath.Join(projectDir, "project.json")

		// Read configuration file
		configData, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read project configuration: %v", err)
		}

		// Parse JSON to validate structure
		var config map[string]interface{}
		if err := json.Unmarshal(configData, &config); err != nil {
			t.Errorf("Project configuration is not valid JSON: %v", err)
		}

		// Check required fields
		requiredFields := []string{"id", "name", "created_at", "updated_at"}
		for _, field := range requiredFields {
			if _, exists := config[field]; !exists {
				t.Errorf("Project configuration missing required field: %s", field)
			}
		}

		// Validate field values
		if config["id"] != projectID {
			t.Errorf("Project ID mismatch: expected %s, got %v", projectID, config["id"])
		}
		if config["name"] != "Validation Test Project" {
			t.Errorf("Project name mismatch: expected 'Validation Test Project', got %v", config["name"])
		}
	})

	// Test 2: Validate task database structure
	t.Run("Task database structure", func(t *testing.T) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("Failed to get home directory: %v", err)
		}
		projectDir := filepath.Join(homeDir, ".laforge", "projects", projectID)
		dbPath := filepath.Join(projectDir, "tasks.db")

		// Open database
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			t.Fatalf("Failed to open task database: %v", err)
		}
		defer db.Close()

		// Test connectivity
		if err := db.Ping(); err != nil {
			t.Errorf("Failed to ping database: %v", err)
		}

		// Run integrity check
		var integrityCheck string
		err = db.QueryRow("PRAGMA integrity_check").Scan(&integrityCheck)
		if err != nil {
			t.Errorf("Failed to run integrity check: %v", err)
		}
		if integrityCheck != "ok" {
			t.Errorf("Database integrity check failed: %s", integrityCheck)
		}

		// Check that required tables exist
		requiredTables := []string{"tasks", "task_logs", "task_reviews"}
		for _, tableName := range requiredTables {
			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&count)
			if err != nil {
				t.Errorf("Failed to check for table %s: %v", tableName, err)
			}
			if count == 0 {
				t.Errorf("Required table %s does not exist", tableName)
			}
		}
	})
}
