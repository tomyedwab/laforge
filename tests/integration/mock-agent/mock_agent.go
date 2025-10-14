package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Mock agent that simulates agent behavior for integration testing
func main() {
	fmt.Println("Mock LaForge Agent Starting...")

	// Get environment variables
	projectID := os.Getenv("LAFORGE_PROJECT_ID")
	stepMode := os.Getenv("LAFORGE_STEP")
	workDir := "/workspace" // Default work directory in container

	fmt.Printf("Project ID: %s\n", projectID)
	fmt.Printf("Step Mode: %s\n", stepMode)
	fmt.Printf("Work Directory: %s\n", workDir)

	// Simulate agent work based on mode
	if stepMode == "true" {
		// In step mode, create some changes to test git functionality
		if err := simulateAgentWork(workDir); err != nil {
			fmt.Printf("Error during agent work: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("Mock LaForge Agent Completed Successfully")
}

func simulateAgentWork(workDir string) error {
	// Create a test file to simulate agent making changes
	testFile := filepath.Join(workDir, "agent_output.txt")
	content := fmt.Sprintf(`Mock Agent Output
================

Generated at: %s
Project: Test Project
Status: Success

This file simulates changes made by the LaForge agent during a step.
`, time.Now().Format(time.RFC3339))

	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write test file: %w", err)
	}

	fmt.Printf("Created test file: %s\n", testFile)

	// Optionally create a task update if TASKS_DB_PATH is set
	if dbPath := os.Getenv("TASKS_DB_PATH"); dbPath != "" {
		fmt.Printf("Would update tasks in database: %s\n", dbPath)
		// In a real mock, we could connect to the database and make changes
	}

	return nil
}
