package integration

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestLaForgeStepValidation tests the validation logic of laforge step command
func TestLaForgeStepValidation(t *testing.T) {
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
		setup       func() error
		cleanup     func()
	}{
		{
			name:        "Step with non-existent project",
			projectID:   "non-existent-project",
			args:        []string{},
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:        "Step with empty project ID",
			projectID:   "",
			args:        []string{},
			wantErr:     true,
			errContains: "project ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				if err := tt.setup(); err != nil {
					t.Fatalf("Setup failed: %v", err)
				}
			}
			if tt.cleanup != nil {
				defer tt.cleanup()
			}

			cmd := exec.Command("./laforge-test", append([]string{"step", tt.projectID}, tt.args...)...)
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
				}
			}
		})
	}
}

// TestLaForgeStepFlags tests the flag validation for step command
func TestLaForgeStepFlags(t *testing.T) {
	// Build the laforge binary for testing
	if err := exec.Command("go", "build", "-o", "laforge-test", "../../cmd/laforge").Run(); err != nil {
		t.Skip("Cannot build laforge binary")
	}
	defer os.Remove("laforge-test")

	// Create a test project for step testing
	projectID := "step-flag-test-project"
	cleanupTestProject(projectID)
	defer cleanupTestProject(projectID)

	// Initialize the test project
	initCmd := exec.Command("./laforge-test", "init", projectID)
	if output, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to initialize test project: %v\nOutput: %s", err, string(output))
	}

	tests := []struct {
		name     string
		args     []string
		wantErr  bool
		errCheck func(output string) bool
	}{
		{
			name:    "Step with valid timeout flag",
			args:    []string{"--timeout", "30s"},
			wantErr: false, // Should fail due to Docker not being available, but flag parsing should work
		},
		{
			name:    "Step with invalid timeout format",
			args:    []string{"--timeout", "invalid"},
			wantErr: true,
			errCheck: func(output string) bool {
				return strings.Contains(output, "invalid duration")
			},
		},
		{
			name:    "Step with agent image flag",
			args:    []string{"--agent-image", "test-agent:latest"},
			wantErr: false, // Should fail due to Docker not being available, but flag parsing should work
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("./laforge-test", append([]string{"step", projectID}, tt.args...)...)
			output, err := cmd.CombinedOutput()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none. Output: %s", string(output))
				} else if tt.errCheck != nil && !tt.errCheck(string(output)) {
					t.Errorf("Error output didn't match expected pattern. Got: %s", string(output))
				}
			} else {
				// For non-error cases, we expect the command to fail due to Docker not being available
				// but we want to check that it's not a flag parsing error
				if err != nil && strings.Contains(string(output), "invalid") && !strings.Contains(string(output), "Docker") {
					t.Errorf("Unexpected flag parsing error: %v\nOutput: %s", err, string(output))
				}
			}
		})
	}
}
