package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestLaForgeInitBasic tests the basic functionality of laforge init command
func TestLaForgeInitBasic(t *testing.T) {
	// Build the laforge binary for testing
	if err := exec.Command("go", "build", "-o", "laforge-test", "../../cmd/laforge").Run(); err != nil {
		t.Skip("Cannot build laforge binary")
	}
	defer os.Remove("laforge-test")

	tests := []struct {
		name        string
		projectID   string
		args        []string
		wantErr     bool
		errContains string
		validate    func(t *testing.T, projectID string)
	}{
		{
			name:      "Init with minimal args",
			projectID: "test-project-basic",
			args:      []string{},
			wantErr:   false,
			validate:  validateBasicProjectStructure,
		},
		{
			name:      "Init with name and description",
			projectID: "test-project-metadata",
			args:      []string{"--name", "Test Project", "--description", "A test project"},
			wantErr:   false,
			validate:  validateProjectWithMetadata,
		},
		{
			name:        "Init with empty project ID",
			projectID:   "",
			args:        []string{},
			wantErr:     true,
			errContains: "project ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing test project
			if tt.projectID != "" {
				cleanupTestProject(tt.projectID)
			}

			cmd := exec.Command("./laforge-test", append([]string{"init", tt.projectID}, tt.args...)...)
			output, err := cmd.CombinedOutput()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none. Output: %s", string(output))
				} else if tt.errContains != "" && !strings.Contains(string(output), tt.errContains) {
					t.Errorf("Expected error containing '%s', got: %s", tt.errContains, string(output))
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v\nOutput: %s", err, string(output))
				} else if tt.validate != nil {
					tt.validate(t, tt.projectID)
				}
			}

			// Clean up
			if !tt.wantErr && tt.projectID != "" {
				defer cleanupTestProject(tt.projectID)
			}
		})
	}
}

// TestLaForgeInitDuplicate tests duplicate project initialization
func TestLaForgeInitDuplicate(t *testing.T) {
	// Build the laforge binary for testing
	if err := exec.Command("go", "build", "-o", "laforge-test", "../../cmd/laforge").Run(); err != nil {
		t.Skip("Cannot build laforge binary")
	}
	defer os.Remove("laforge-test")

	projectID := "duplicate-test-project"
	cleanupTestProject(projectID)
	defer cleanupTestProject(projectID)

	// First initialization should succeed
	cmd := exec.Command("./laforge-test", "init", projectID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("First initialization failed: %v\nOutput: %s", err, string(output))
	}

	// Second initialization should fail
	cmd = exec.Command("./laforge-test", "init", projectID)
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected error for duplicate project initialization, but got none. Output: %s", string(output))
	} else if !strings.Contains(string(output), "already exists") {
		t.Errorf("Expected 'already exists' error, got: %s", string(output))
	}
}

// TestLaForgeInitFlags tests various flag combinations
func TestLaForgeInitFlags(t *testing.T) {
	// Build the laforge binary for testing
	if err := exec.Command("go", "build", "-o", "laforge-test", "../../cmd/laforge").Run(); err != nil {
		t.Skip("Cannot build laforge binary")
	}
	defer os.Remove("laforge-test")

	tests := []struct {
		name      string
		projectID string
		args      []string
		validate  func(t *testing.T, projectID string)
	}{
		{
			name:      "Init with only name flag",
			projectID: "test-name-flag",
			args:      []string{"--name", "My Test Project"},
			validate:  validateProjectName,
		},
		{
			name:      "Init with only description flag",
			projectID: "test-desc-flag",
			args:      []string{"--description", "Project with description only"},
			validate:  validateProjectDescription,
		},
		{
			name:      "Init with both flags",
			projectID: "test-both-flags",
			args:      []string{"--name", "Full Test Project", "--description", "Project with both name and description"},
			validate:  validateProjectFullMetadata,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanupTestProject(tt.projectID)
			defer cleanupTestProject(tt.projectID)

			cmd := exec.Command("./laforge-test", append([]string{"init", tt.projectID}, tt.args...)...)
			output, err := cmd.CombinedOutput()

			if err != nil {
				t.Errorf("Unexpected error: %v\nOutput: %s", err, string(output))
			} else if tt.validate != nil {
				tt.validate(t, tt.projectID)
			}
		})
	}
}

// Validation functions
func validateBasicProjectStructure(t *testing.T, projectID string) {
	// Get project directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}
	projectDir := filepath.Join(homeDir, ".laforge", "projects", projectID)

	// Check project directory exists
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		t.Errorf("Project directory does not exist: %s", projectDir)
	}

	// Check project configuration file
	configPath := filepath.Join(projectDir, "project.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Project configuration file does not exist: %s", configPath)
	}

	// Check task database
	dbPath := filepath.Join(projectDir, "tasks.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Task database does not exist: %s", dbPath)
	}

	// Check git repository (should be initialized if git is available)
	gitPath := filepath.Join(projectDir, ".git")
	if _, err := os.Stat(gitPath); os.IsNotExist(err) {
		// Git repository initialization is optional, so we only warn if git is available
		if isGitAvailable() {
			t.Errorf("Git repository not initialized: %s", gitPath)
		}
	}
}

func validateProjectWithMetadata(t *testing.T, projectID string) {
	validateBasicProjectStructure(t, projectID)

	// Additional validation for projects with metadata
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}
	projectDir := filepath.Join(homeDir, ".laforge", "projects", projectID)
	configPath := filepath.Join(projectDir, "project.json")

	// Read and validate configuration
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read project configuration: %v", err)
	}

	// Basic validation that the config contains expected metadata
	configStr := string(configData)
	if !strings.Contains(configStr, "Test Project") {
		t.Errorf("Project configuration does not contain expected name")
	}
	if !strings.Contains(configStr, "A test project") {
		t.Errorf("Project configuration does not contain expected description")
	}
}

func validateProjectName(t *testing.T, projectID string) {
	validateBasicProjectStructure(t, projectID)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}
	projectDir := filepath.Join(homeDir, ".laforge", "projects", projectID)
	configPath := filepath.Join(projectDir, "project.json")

	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read project configuration: %v", err)
	}

	if !strings.Contains(string(configData), "My Test Project") {
		t.Errorf("Project configuration does not contain expected name")
	}
}

func validateProjectDescription(t *testing.T, projectID string) {
	validateBasicProjectStructure(t, projectID)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}
	projectDir := filepath.Join(homeDir, ".laforge", "projects", projectID)
	configPath := filepath.Join(projectDir, "project.json")

	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read project configuration: %v", err)
	}

	if !strings.Contains(string(configData), "Project with description only") {
		t.Errorf("Project configuration does not contain expected description")
	}
}

func validateProjectFullMetadata(t *testing.T, projectID string) {
	validateBasicProjectStructure(t, projectID)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}
	projectDir := filepath.Join(homeDir, ".laforge", "projects", projectID)
	configPath := filepath.Join(projectDir, "project.json")

	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read project configuration: %v", err)
	}

	configStr := string(configData)
	if !strings.Contains(configStr, "Full Test Project") {
		t.Errorf("Project configuration does not contain expected name")
	}
	if !strings.Contains(configStr, "Project with both name and description") {
		t.Errorf("Project configuration does not contain expected description")
	}
}

func cleanupTestProject(projectID string) {
	// Get project directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	projectDir := filepath.Join(homeDir, ".laforge", "projects", projectID)

	// Remove project directory if it exists
	if _, err := os.Stat(projectDir); err == nil {
		os.RemoveAll(projectDir)
	}
}

func isGitAvailable() bool {
	cmd := exec.Command("git", "--version")
	err := cmd.Run()
	return err == nil
}
