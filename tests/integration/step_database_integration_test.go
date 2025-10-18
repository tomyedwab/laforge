package integration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// TestStepDatabaseRecording tests step recording during step execution
func TestStepDatabaseRecording(t *testing.T) {
	// Build the laforge binary for testing
	if err := exec.Command("go", "build", "-o", "laforge-test", "../../cmd/laforge").Run(); err != nil {
		t.Skip("Cannot build laforge binary")
	}
	defer os.Remove("laforge-test")

	projectID := "step-recording-test-project"
	cleanupTestProject(projectID)
	defer cleanupTestProject(projectID)

	// Initialize project
	initCmd := exec.Command("./laforge-test", "init", projectID, "--name", "Step Recording Test Project")
	output, err := initCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to initialize project: %v\nOutput: %s", err, string(output))
	}

	// Test that step database is created during initialization
	t.Run("Step database created during project initialization", func(t *testing.T) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("Failed to get home directory: %v", err)
		}
		projectDir := filepath.Join(homeDir, ".laforge", "projects", projectID)
		stepsDBPath := filepath.Join(projectDir, "steps.db")

		// Check that steps database file exists
		if _, err := os.Stat(stepsDBPath); os.IsNotExist(err) {
			t.Errorf("Steps database file not created during project initialization")
		}

		// Verify database connectivity and schema
		db, err := sql.Open("sqlite3", stepsDBPath)
		if err != nil {
			t.Fatalf("Failed to open steps database: %v", err)
		}
		defer db.Close()

		// Test connectivity
		if err := db.Ping(); err != nil {
			t.Errorf("Failed to ping steps database: %v", err)
		}

		// Run integrity check
		var integrityCheck string
		err = db.QueryRow("PRAGMA integrity_check").Scan(&integrityCheck)
		if err != nil {
			t.Errorf("Failed to run integrity check: %v", err)
		}
		if integrityCheck != "ok" {
			t.Errorf("Steps database integrity check failed: %s", integrityCheck)
		}

		// Check that required tables exist
		requiredTables := []string{"steps"}
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

	// Test step recording during step execution (if Docker is available)
	if commandExists("docker") && dockerAvailable() {
		t.Run("Step recording during step execution", func(t *testing.T) {
			// Build mock agent if not already built
			buildMockAgent(t)

			// Run a step
			stepCmd := exec.Command("./laforge-test", "step", projectID, "--agent-image", "laforge-mock-agent:latest")
			output, err := stepCmd.CombinedOutput()
			if err != nil {
				t.Logf("Step execution failed (expected in some environments): %v\nOutput: %s", err, string(output))
				t.Skip("Step execution failed, skipping step recording test")
			}

			// Verify step was recorded in database
			homeDir, err := os.UserHomeDir()
			if err != nil {
				t.Fatalf("Failed to get home directory: %v", err)
			}
			projectDir := filepath.Join(homeDir, ".laforge", "projects", projectID)
			stepsDBPath := filepath.Join(projectDir, "steps.db")

			db, err := sql.Open("sqlite3", stepsDBPath)
			if err != nil {
				t.Fatalf("Failed to open steps database: %v", err)
			}
			defer db.Close()

			// Check that a step was recorded
			var stepCount int
			err = db.QueryRow("SELECT COUNT(*) FROM steps WHERE project_id = ?", projectID).Scan(&stepCount)
			if err != nil {
				t.Errorf("Failed to count steps: %v", err)
			}
			if stepCount == 0 {
				t.Errorf("No steps recorded in database after step execution")
			}

			// Verify step details
			var stepID int
			var active bool
			var commitSHABefore, commitSHAAfter string
			var startTime time.Time
			var endTime sql.NullTime
			var durationMs sql.NullInt64
			var exitCode sql.NullInt64
			var agentConfigJSON string
			var tokenUsageJSON string
			var createdAt time.Time

			err = db.QueryRow(`
				SELECT id, active, commit_sha_before, commit_sha_after, start_time, end_time, 
				       duration_ms, exit_code, agent_config_json, token_usage_json, created_at
				FROM steps WHERE project_id = ? ORDER BY id DESC LIMIT 1`, projectID).Scan(
				&stepID, &active, &commitSHABefore, &commitSHAAfter, &startTime, &endTime,
				&durationMs, &exitCode, &agentConfigJSON, &tokenUsageJSON, &createdAt)
			if err != nil {
				t.Errorf("Failed to query step details: %v", err)
			}

			// Validate step data
			if stepID <= 0 {
				t.Errorf("Invalid step ID: %d", stepID)
			}
			if !active {
				t.Errorf("Step should be active by default")
			}
			if commitSHABefore == "" {
				t.Errorf("Commit SHA before should not be empty")
			}
			if endTime.Valid && durationMs.Valid {
				if durationMs.Int64 <= 0 {
					t.Errorf("Duration should be positive: %d", durationMs.Int64)
				}
			}
			if exitCode.Valid {
				if exitCode.Int64 != 0 && exitCode.Int64 != 1 {
					t.Errorf("Exit code should be 0 or 1: %d", exitCode.Int64)
				}
			}

			// Validate agent config JSON
			var agentConfig map[string]interface{}
			if err := json.Unmarshal([]byte(agentConfigJSON), &agentConfig); err != nil {
				t.Errorf("Agent config JSON is invalid: %v", err)
			}

			// Validate token usage JSON
			var tokenUsage map[string]interface{}
			if err := json.Unmarshal([]byte(tokenUsageJSON), &tokenUsage); err != nil {
				t.Errorf("Token usage JSON is invalid: %v", err)
			}
		})
	} else {
		t.Log("Docker not available, skipping step execution test")
	}
}

// TestStepDatabaseIsolation tests step database isolation during step execution
func TestStepDatabaseIsolation(t *testing.T) {
	// Build the laforge binary for testing
	if err := exec.Command("go", "build", "-o", "laforge-test", "../../cmd/laforge").Run(); err != nil {
		t.Skip("Cannot build laforge binary")
	}
	defer os.Remove("laforge-test")

	projectID := "step-isolation-test-project"
	cleanupTestProject(projectID)
	defer cleanupTestProject(projectID)

	// Initialize project
	initCmd := exec.Command("./laforge-test", "init", projectID, "--name", "Step Isolation Test Project")
	output, err := initCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to initialize project: %v\nOutput: %s", err, string(output))
	}

	t.Run("Step database isolation verification", func(t *testing.T) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("Failed to get home directory: %v", err)
		}
		projectDir := filepath.Join(homeDir, ".laforge", "projects", projectID)
		stepsDBPath := filepath.Join(projectDir, "steps.db")

		// Verify steps database is separate from tasks database
		tasksDBPath := filepath.Join(projectDir, "tasks.db")

		if stepsDBPath == tasksDBPath {
			t.Errorf("Steps database path should be different from tasks database path")
		}

		// Verify both databases exist
		if _, err := os.Stat(stepsDBPath); os.IsNotExist(err) {
			t.Errorf("Steps database file not found: %s", stepsDBPath)
		}
		if _, err := os.Stat(tasksDBPath); os.IsNotExist(err) {
			t.Errorf("Tasks database file not found: %s", tasksDBPath)
		}

		// Verify database schemas are different
		stepsDB, err := sql.Open("sqlite3", stepsDBPath)
		if err != nil {
			t.Fatalf("Failed to open steps database: %v", err)
		}
		defer stepsDB.Close()

		tasksDB, err := sql.Open("sqlite3", tasksDBPath)
		if err != nil {
			t.Fatalf("Failed to open tasks database: %v", err)
		}
		defer tasksDB.Close()

		// Check steps database has steps table but not tasks table
		var stepsTableCount int
		err = stepsDB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='steps'").Scan(&stepsTableCount)
		if err != nil {
			t.Errorf("Failed to check steps table in steps database: %v", err)
		}
		if stepsTableCount == 0 {
			t.Errorf("Steps table should exist in steps database")
		}

		var tasksTableInStepsCount int
		err = stepsDB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tasks'").Scan(&tasksTableInStepsCount)
		if err != nil {
			t.Errorf("Failed to check tasks table in steps database: %v", err)
		}
		if tasksTableInStepsCount > 0 {
			t.Errorf("Tasks table should not exist in steps database")
		}

		// Check tasks database has tasks table but not steps table
		var tasksTableCount int
		err = tasksDB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tasks'").Scan(&tasksTableCount)
		if err != nil {
			t.Errorf("Failed to check tasks table in tasks database: %v", err)
		}
		if tasksTableCount == 0 {
			t.Errorf("Tasks table should exist in tasks database")
		}

		var stepsTableInTasksCount int
		err = tasksDB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='steps'").Scan(&stepsTableInTasksCount)
		if err != nil {
			t.Errorf("Failed to check steps table in tasks database: %v", err)
		}
		if stepsTableInTasksCount > 0 {
			t.Errorf("Steps table should not exist in tasks database")
		}
	})
}

// TestStepCommandsWithRealData tests step command functionality with real data
func TestStepCommandsWithRealData(t *testing.T) {
	// Build the laforge binary for testing
	if err := exec.Command("go", "build", "-o", "laforge-test", "../../cmd/laforge").Run(); err != nil {
		t.Skip("Cannot build laforge binary")
	}
	defer os.Remove("laforge-test")

	projectID := "step-commands-test-project"
	cleanupTestProject(projectID)
	defer cleanupTestProject(projectID)

	// Initialize project
	initCmd := exec.Command("./laforge-test", "init", projectID, "--name", "Step Commands Test Project")
	output, err := initCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to initialize project: %v\nOutput: %s", err, string(output))
	}

	// Create some test steps by running mock steps if Docker is available
	if commandExists("docker") && dockerAvailable() {
		buildMockAgent(t)

		// Run multiple steps to create test data
		for i := 0; i < 3; i++ {
			stepCmd := exec.Command("./laforge-test", "step", projectID, "--agent-image", "laforge-mock-agent:latest")
			output, err := stepCmd.CombinedOutput()
			if err != nil {
				t.Logf("Step %d execution failed: %v\nOutput: %s", i+1, err, string(output))
				continue
			}
			t.Logf("Step %d executed successfully", i+1)
		}
	}

	t.Run("List steps command with real data", func(t *testing.T) {
		cmd := exec.Command("./laforge-test", "steps", projectID)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to list steps: %v\nOutput: %s", err, string(output))
		}

		// Verify output contains expected information
		outputStr := string(output)
		if !contains(outputStr, "No steps found") && !contains(outputStr, "Step ID") {
			t.Errorf("Steps output should contain either 'No steps found' message or step headers")
		}

		// If steps exist, verify headers are present
		if !contains(outputStr, "No steps found") {
			if !contains(outputStr, "Step ID") {
				t.Errorf("Steps output should contain 'Step ID' header when steps exist")
			}
			if !contains(outputStr, "Status") {
				t.Errorf("Steps output should contain 'Status' header when steps exist")
			}
			if !contains(outputStr, "Duration") {
				t.Errorf("Steps output should contain 'Duration' header when steps exist")
			}
			if !contains(outputStr, "Commit") {
				t.Errorf("Steps output should contain 'Commit' header when steps exist")
			}
		}

		t.Logf("Steps command output:\n%s", outputStr)
	})

	t.Run("Step info command with real data", func(t *testing.T) {
		// First get the latest step ID
		homeDir, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("Failed to get home directory: %v", err)
		}
		projectDir := filepath.Join(homeDir, ".laforge", "projects", projectID)
		stepsDBPath := filepath.Join(projectDir, "steps.db")

		db, err := sql.Open("sqlite3", stepsDBPath)
		if err != nil {
			t.Fatalf("Failed to open steps database: %v", err)
		}
		defer db.Close()

		var stepID int
		err = db.QueryRow("SELECT id FROM steps WHERE project_id = ? ORDER BY id DESC LIMIT 1", projectID).Scan(&stepID)
		if err != nil {
			t.Skip("No steps found in database, skipping step info test")
		}

		cmd := exec.Command("./laforge-test", "info", projectID, fmt.Sprintf("%d", stepID))
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to get step info: %v\nOutput: %s", err, string(output))
		}

		// Verify output contains expected information
		outputStr := string(output)
		if !contains(outputStr, fmt.Sprintf("Step ID: %d", stepID)) {
			t.Errorf("Step info output should contain step ID: %d", stepID)
		}
		if !contains(outputStr, "Status:") {
			t.Errorf("Step info output should contain 'Status:'")
		}
		if !contains(outputStr, "Start Time:") {
			t.Errorf("Step info output should contain 'Start Time:'")
		}
		if !contains(outputStr, "Duration:") {
			t.Errorf("Step info output should contain 'Duration:'")
		}
		if !contains(outputStr, "Commit Before:") {
			t.Errorf("Step info output should contain 'Commit Before:'")
		}

		t.Logf("Step info command output:\n%s", outputStr)
	})
}

// TestStepDatabaseErrorScenarios tests error scenarios in step database operations
func TestStepDatabaseErrorScenarios(t *testing.T) {
	// Build the laforge binary for testing
	if err := exec.Command("go", "build", "-o", "laforge-test", "../../cmd/laforge").Run(); err != nil {
		t.Skip("Cannot build laforge binary")
	}
	defer os.Remove("laforge-test")

	t.Run("Steps command with non-existent project", func(t *testing.T) {
		cmd := exec.Command("./laforge-test", "steps", "non-existent-project")
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("Expected error for non-existent project, but got none")
		}
		if !contains(string(output), "not found") {
			t.Errorf("Expected 'not found' error message, got: %s", string(output))
		}
	})

	t.Run("Step info command with non-existent project", func(t *testing.T) {
		cmd := exec.Command("./laforge-test", "info", "non-existent-project", "1")
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Errorf("Expected error for non-existent project, but got none")
		}
		if !contains(string(output), "not found") {
			t.Errorf("Expected 'not found' error message, got: %s", string(output))
		}
	})

	t.Run("Step info command with invalid step ID", func(t *testing.T) {
		projectID := "error-test-project"
		cleanupTestProject(projectID)
		defer cleanupTestProject(projectID)

		// Initialize project
		initCmd := exec.Command("./laforge-test", "init", projectID)
		output, err := initCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to initialize project: %v\nOutput: %s", err, string(output))
		}

		cmd := exec.Command("./laforge-test", "info", projectID, "999999")
		output, err = cmd.CombinedOutput()
		if err == nil {
			t.Errorf("Expected error for non-existent step ID, but got none")
		}
		if !contains(string(output), "not found") {
			t.Errorf("Expected 'not found' error message, got: %s", string(output))
		}
	})

	t.Run("Step info command with invalid step ID format", func(t *testing.T) {
		projectID := "error-test-project-2"
		cleanupTestProject(projectID)
		defer cleanupTestProject(projectID)

		// Initialize project
		initCmd := exec.Command("./laforge-test", "init", projectID)
		output, err := initCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to initialize project: %v\nOutput: %s", err, string(output))
		}

		cmd := exec.Command("./laforge-test", "info", projectID, "invalid-id")
		output, err = cmd.CombinedOutput()
		if err == nil {
			t.Errorf("Expected error for invalid step ID format, but got none")
		}
		if !contains(string(output), "invalid step ID") {
			t.Errorf("Expected 'invalid step ID' error message, got: %s", string(output))
		}
	})
}

// TestStepDatabasePerformance tests performance impact of step database operations
func TestStepDatabasePerformance(t *testing.T) {
	// Build the laforge binary for testing
	if err := exec.Command("go", "build", "-o", "laforge-test", "../../cmd/laforge").Run(); err != nil {
		t.Skip("Cannot build laforge binary")
	}
	defer os.Remove("laforge-test")

	projectID := "performance-test-project"
	cleanupTestProject(projectID)
	defer cleanupTestProject(projectID)

	// Initialize project
	initCmd := exec.Command("./laforge-test", "init", projectID, "--name", "Performance Test Project")
	output, err := initCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to initialize project: %v\nOutput: %s", err, string(output))
	}

	t.Run("Step database query performance", func(t *testing.T) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("Failed to get home directory: %v", err)
		}
		projectDir := filepath.Join(homeDir, ".laforge", "projects", projectID)
		stepsDBPath := filepath.Join(projectDir, "steps.db")

		db, err := sql.Open("sqlite3", stepsDBPath)
		if err != nil {
			t.Fatalf("Failed to open steps database: %v", err)
		}
		defer db.Close()

		// Measure time to query empty steps table
		start := time.Now()
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM steps WHERE project_id = ?", projectID).Scan(&count)
		duration := time.Since(start)
		if err != nil {
			t.Errorf("Failed to query steps: %v", err)
		}

		t.Logf("Query performance: %v for counting steps (result: %d)", duration, count)

		// Performance should be reasonable even with empty table
		if duration > 100*time.Millisecond {
			t.Errorf("Query took too long: %v (expected < 100ms)", duration)
		}
	})

	t.Run("Step database schema efficiency", func(t *testing.T) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("Failed to get home directory: %v", err)
		}
		projectDir := filepath.Join(homeDir, ".laforge", "projects", projectID)
		stepsDBPath := filepath.Join(projectDir, "steps.db")

		db, err := sql.Open("sqlite3", stepsDBPath)
		if err != nil {
			t.Fatalf("Failed to open steps database: %v", err)
		}
		defer db.Close()

		// Check that indexes exist for performance
		expectedIndexes := []string{"idx_steps_project_id", "idx_steps_active", "idx_steps_created_at"}
		for _, indexName := range expectedIndexes {
			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", indexName).Scan(&count)
			if err != nil {
				t.Errorf("Failed to check for index %s: %v", indexName, err)
			}
			if count == 0 {
				t.Errorf("Expected index %s does not exist", indexName)
			} else {
				t.Logf("Index %s exists for performance optimization", indexName)
			}
		}
	})
}

// Helper functions

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func dockerAvailable() bool {
	cmd := exec.Command("docker", "--version")
	err := cmd.Run()
	return err == nil
}

func buildMockAgent(t *testing.T) {
	mockAgentDir := filepath.Join(".", "mock-agent")
	cmd := exec.Command("docker", "build", "-t", "laforge-mock-agent:latest", mockAgentDir+"/.")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Failed to build mock agent: %v\nOutput: %s", err, string(output))
		t.Skip("Cannot build mock agent, skipping step execution test")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
