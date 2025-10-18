package steps

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) (*StepDatabase, func()) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "stepdb-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "steps.db")
	sdb, err := InitStepDB(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to initialize step database: %v", err)
	}

	cleanup := func() {
		sdb.Close()
		os.RemoveAll(tempDir)
	}

	return sdb, cleanup
}

func TestInitStepDB(t *testing.T) {
	sdb, cleanup := setupTestDB(t)
	defer cleanup()

	if sdb == nil {
		t.Fatal("StepDatabase should not be nil")
	}

	if sdb.GetDB() == nil {
		t.Fatal("Underlying database should not be nil")
	}
}

func TestCreateStep(t *testing.T) {
	sdb, cleanup := setupTestDB(t)
	defer cleanup()

	agentConfig := AgentConfig{
		Model:        "test-model",
		MaxTokens:    1000,
		Temperature:  0.7,
		SystemPrompt: "test prompt",
		Tools:        []string{"tool1", "tool2"},
		Metadata:     map[string]string{"key": "value"},
	}

	tokenUsage := TokenUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		Cost:             0.001,
	}

	step := &Step{
		Active:          true,
		ParentStepID:    nil,
		CommitSHABefore: "abc123",
		CommitSHAAfter:  "",
		AgentConfig:     agentConfig,
		StartTime:       time.Now(),
		EndTime:         nil,
		DurationMs:      nil,
		TokenUsage:      tokenUsage,
		ExitCode:        nil,
		ProjectID:       "test-project",
		CreatedAt:       time.Now(),
	}

	err := sdb.CreateStep(step)
	if err != nil {
		t.Fatalf("Failed to create step: %v", err)
	}

	if step.ID == 0 {
		t.Fatal("Step ID should not be 0 after creation")
	}
}

func TestGetStep(t *testing.T) {
	sdb, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a test step
	agentConfig := AgentConfig{
		Model:        "test-model",
		MaxTokens:    1000,
		Temperature:  0.7,
		SystemPrompt: "test prompt",
		Tools:        []string{"tool1", "tool2"},
		Metadata:     map[string]string{"key": "value"},
	}

	tokenUsage := TokenUsage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		Cost:             0.001,
	}

	originalStep := &Step{
		Active:          true,
		ParentStepID:    nil,
		CommitSHABefore: "abc123",
		CommitSHAAfter:  "def456",
		AgentConfig:     agentConfig,
		StartTime:       time.Now(),
		EndTime:         nil,
		DurationMs:      nil,
		TokenUsage:      tokenUsage,
		ExitCode:        nil,
		ProjectID:       "test-project",
		CreatedAt:       time.Now(),
	}

	err := sdb.CreateStep(originalStep)
	if err != nil {
		t.Fatalf("Failed to create step: %v", err)
	}

	// Retrieve the step
	retrievedStep, err := sdb.GetStep(originalStep.ID)
	if err != nil {
		t.Fatalf("Failed to get step: %v", err)
	}

	if retrievedStep == nil {
		t.Fatal("Retrieved step should not be nil")
	}

	// Verify fields
	if retrievedStep.ID != originalStep.ID {
		t.Errorf("Step ID mismatch: got %d, want %d", retrievedStep.ID, originalStep.ID)
	}

	if retrievedStep.ProjectID != originalStep.ProjectID {
		t.Errorf("Project ID mismatch: got %s, want %s", retrievedStep.ProjectID, originalStep.ProjectID)
	}

	if retrievedStep.CommitSHABefore != originalStep.CommitSHABefore {
		t.Errorf("Commit SHA before mismatch: got %s, want %s", retrievedStep.CommitSHABefore, originalStep.CommitSHABefore)
	}

	if retrievedStep.AgentConfig.Model != originalStep.AgentConfig.Model {
		t.Errorf("Agent config model mismatch: got %s, want %s", retrievedStep.AgentConfig.Model, originalStep.AgentConfig.Model)
	}

	if retrievedStep.TokenUsage.TotalTokens != originalStep.TokenUsage.TotalTokens {
		t.Errorf("Token usage mismatch: got %d, want %d", retrievedStep.TokenUsage.TotalTokens, originalStep.TokenUsage.TotalTokens)
	}
}

func TestGetStepNotFound(t *testing.T) {
	sdb, cleanup := setupTestDB(t)
	defer cleanup()

	step, err := sdb.GetStep(999)
	if err != nil {
		t.Fatalf("Failed to get step: %v", err)
	}

	if step != nil {
		t.Fatal("Step should be nil when not found")
	}
}

func TestGetLatestActiveStep(t *testing.T) {
	sdb, cleanup := setupTestDB(t)
	defer cleanup()

	projectID := "test-project"

	// Create multiple steps
	for i := 0; i < 3; i++ {
		step := &Step{
			Active:          true,
			ParentStepID:    nil,
			CommitSHABefore: "abc123",
			CommitSHAAfter:  "",
			AgentConfig: AgentConfig{
				Model:        "test-model",
				MaxTokens:    1000,
				Temperature:  0.7,
				SystemPrompt: "test prompt",
			},
			StartTime:  time.Now(),
			EndTime:    nil,
			DurationMs: nil,
			TokenUsage: TokenUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				Cost:             0.001,
			},
			ExitCode:  nil,
			ProjectID: projectID,
			CreatedAt: time.Now(),
		}

		err := sdb.CreateStep(step)
		if err != nil {
			t.Fatalf("Failed to create step %d: %v", i, err)
		}
	}

	// Get latest active step
	latestStep, err := sdb.GetLatestActiveStep(projectID)
	if err != nil {
		t.Fatalf("Failed to get latest active step: %v", err)
	}

	if latestStep == nil {
		t.Fatal("Latest step should not be nil")
	}

	if latestStep.ID != 3 {
		t.Errorf("Expected latest step ID to be 3, got %d", latestStep.ID)
	}
}

func TestUpdateStep(t *testing.T) {
	sdb, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a step
	step := &Step{
		Active:          true,
		ParentStepID:    nil,
		CommitSHABefore: "abc123",
		CommitSHAAfter:  "",
		AgentConfig: AgentConfig{
			Model:        "test-model",
			MaxTokens:    1000,
			Temperature:  0.7,
			SystemPrompt: "test prompt",
		},
		StartTime:  time.Now(),
		EndTime:    nil,
		DurationMs: nil,
		TokenUsage: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Cost:             0.001,
		},
		ExitCode:  nil,
		ProjectID: "test-project",
		CreatedAt: time.Now(),
	}

	err := sdb.CreateStep(step)
	if err != nil {
		t.Fatalf("Failed to create step: %v", err)
	}

	// Update the step
	endTime := time.Now()
	durationMs := 5000
	exitCode := 0
	newTokenUsage := TokenUsage{
		PromptTokens:     200,
		CompletionTokens: 100,
		TotalTokens:      300,
		Cost:             0.002,
	}

	err = sdb.UpdateStep(step.ID, "def456", endTime, durationMs, exitCode, newTokenUsage)
	if err != nil {
		t.Fatalf("Failed to update step: %v", err)
	}

	// Retrieve and verify
	updatedStep, err := sdb.GetStep(step.ID)
	if err != nil {
		t.Fatalf("Failed to get updated step: %v", err)
	}

	if updatedStep.CommitSHAAfter != "def456" {
		t.Errorf("Commit SHA after mismatch: got %s, want def456", updatedStep.CommitSHAAfter)
	}

	if updatedStep.EndTime == nil || !updatedStep.EndTime.Equal(endTime) {
		t.Errorf("End time mismatch")
	}

	if updatedStep.DurationMs == nil || *updatedStep.DurationMs != durationMs {
		t.Errorf("Duration mismatch: got %v, want %d", updatedStep.DurationMs, durationMs)
	}

	if updatedStep.ExitCode == nil || *updatedStep.ExitCode != exitCode {
		t.Errorf("Exit code mismatch: got %v, want %d", updatedStep.ExitCode, exitCode)
	}

	if updatedStep.TokenUsage.TotalTokens != newTokenUsage.TotalTokens {
		t.Errorf("Token usage mismatch: got %d, want %d", updatedStep.TokenUsage.TotalTokens, newTokenUsage.TotalTokens)
	}
}

func TestDeactivateStep(t *testing.T) {
	sdb, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a step
	step := &Step{
		Active:          true,
		ParentStepID:    nil,
		CommitSHABefore: "abc123",
		CommitSHAAfter:  "",
		AgentConfig: AgentConfig{
			Model:        "test-model",
			MaxTokens:    1000,
			Temperature:  0.7,
			SystemPrompt: "test prompt",
		},
		StartTime:  time.Now(),
		EndTime:    nil,
		DurationMs: nil,
		TokenUsage: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Cost:             0.001,
		},
		ExitCode:  nil,
		ProjectID: "test-project",
		CreatedAt: time.Now(),
	}

	err := sdb.CreateStep(step)
	if err != nil {
		t.Fatalf("Failed to create step: %v", err)
	}

	// Deactivate the step
	err = sdb.DeactivateStep(step.ID)
	if err != nil {
		t.Fatalf("Failed to deactivate step: %v", err)
	}

	// Retrieve and verify
	deactivatedStep, err := sdb.GetStep(step.ID)
	if err != nil {
		t.Fatalf("Failed to get deactivated step: %v", err)
	}

	if deactivatedStep.Active {
		t.Error("Step should be inactive after deactivation")
	}
}

func TestListSteps(t *testing.T) {
	sdb, cleanup := setupTestDB(t)
	defer cleanup()

	projectID := "test-project"

	// Create multiple steps
	for i := 0; i < 3; i++ {
		step := &Step{
			Active:          true,
			ParentStepID:    nil,
			CommitSHABefore: "abc123",
			CommitSHAAfter:  "",
			AgentConfig: AgentConfig{
				Model:        "test-model",
				MaxTokens:    1000,
				Temperature:  0.7,
				SystemPrompt: "test prompt",
			},
			StartTime:  time.Now(),
			EndTime:    nil,
			DurationMs: nil,
			TokenUsage: TokenUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				Cost:             0.001,
			},
			ExitCode:  nil,
			ProjectID: projectID,
			CreatedAt: time.Now(),
		}

		err := sdb.CreateStep(step)
		if err != nil {
			t.Fatalf("Failed to create step %d: %v", i, err)
		}
	}

	// List all steps
	steps, err := sdb.ListSteps(projectID, false)
	if err != nil {
		t.Fatalf("Failed to list steps: %v", err)
	}

	if len(steps) != 3 {
		t.Errorf("Expected 3 steps, got %d", len(steps))
	}

	// Deactivate one step and test active-only filter
	err = sdb.DeactivateStep(2)
	if err != nil {
		t.Fatalf("Failed to deactivate step: %v", err)
	}

	activeSteps, err := sdb.ListSteps(projectID, true)
	if err != nil {
		t.Fatalf("Failed to list active steps: %v", err)
	}

	if len(activeSteps) != 2 {
		t.Errorf("Expected 2 active steps, got %d", len(activeSteps))
	}
}

func TestGetStepCount(t *testing.T) {
	sdb, cleanup := setupTestDB(t)
	defer cleanup()

	projectID := "test-project"

	// Initially should be 0
	count, err := sdb.GetStepCount(projectID)
	if err != nil {
		t.Fatalf("Failed to get step count: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 steps initially, got %d", count)
	}

	// Create steps
	for i := 0; i < 3; i++ {
		step := &Step{
			Active:          true,
			ParentStepID:    nil,
			CommitSHABefore: "abc123",
			CommitSHAAfter:  "",
			AgentConfig: AgentConfig{
				Model:        "test-model",
				MaxTokens:    1000,
				Temperature:  0.7,
				SystemPrompt: "test prompt",
			},
			StartTime:  time.Now(),
			EndTime:    nil,
			DurationMs: nil,
			TokenUsage: TokenUsage{
				PromptTokens:     100,
				CompletionTokens: 50,
				TotalTokens:      150,
				Cost:             0.001,
			},
			ExitCode:  nil,
			ProjectID: projectID,
			CreatedAt: time.Now(),
		}

		err := sdb.CreateStep(step)
		if err != nil {
			t.Fatalf("Failed to create step %d: %v", i, err)
		}
	}

	// Should now be 3
	count, err = sdb.GetStepCount(projectID)
	if err != nil {
		t.Fatalf("Failed to get step count: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 steps, got %d", count)
	}
}

func TestGetNextStepID(t *testing.T) {
	sdb, cleanup := setupTestDB(t)
	defer cleanup()

	// Initially should be 1
	nextID, err := sdb.GetNextStepID()
	if err != nil {
		t.Fatalf("Failed to get next step ID: %v", err)
	}

	if nextID != 1 {
		t.Errorf("Expected next step ID to be 1, got %d", nextID)
	}

	// Create a step
	step := &Step{
		Active:          true,
		ParentStepID:    nil,
		CommitSHABefore: "abc123",
		CommitSHAAfter:  "",
		AgentConfig: AgentConfig{
			Model:        "test-model",
			MaxTokens:    1000,
			Temperature:  0.7,
			SystemPrompt: "test prompt",
		},
		StartTime:  time.Now(),
		EndTime:    nil,
		DurationMs: nil,
		TokenUsage: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Cost:             0.001,
		},
		ExitCode:  nil,
		ProjectID: "test-project",
		CreatedAt: time.Now(),
	}

	err = sdb.CreateStep(step)
	if err != nil {
		t.Fatalf("Failed to create step: %v", err)
	}

	// Should now be 2
	nextID, err = sdb.GetNextStepID()
	if err != nil {
		t.Fatalf("Failed to get next step ID: %v", err)
	}

	if nextID != 2 {
		t.Errorf("Expected next step ID to be 2, got %d", nextID)
	}
}

func TestCreateStepValidation(t *testing.T) {
	sdb, cleanup := setupTestDB(t)
	defer cleanup()

	// Test nil step
	err := sdb.CreateStep(nil)
	if err == nil {
		t.Error("Expected error for nil step")
	}

	// Test empty project ID
	step := &Step{
		Active:          true,
		ParentStepID:    nil,
		CommitSHABefore: "abc123",
		CommitSHAAfter:  "",
		AgentConfig: AgentConfig{
			Model:        "test-model",
			MaxTokens:    1000,
			Temperature:  0.7,
			SystemPrompt: "test prompt",
		},
		StartTime:  time.Now(),
		EndTime:    nil,
		DurationMs: nil,
		TokenUsage: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Cost:             0.001,
		},
		ExitCode:  nil,
		ProjectID: "", // Empty project ID
		CreatedAt: time.Now(),
	}

	err = sdb.CreateStep(step)
	if err == nil {
		t.Error("Expected error for empty project ID")
	}

	// Test empty commit SHA before
	step.ProjectID = "test-project"
	step.CommitSHABefore = "" // Empty commit SHA

	err = sdb.CreateStep(step)
	if err == nil {
		t.Error("Expected error for empty commit SHA before")
	}
}

func TestParentStepRelationship(t *testing.T) {
	sdb, cleanup := setupTestDB(t)
	defer cleanup()

	// Create parent step
	parentStep := &Step{
		Active:          true,
		ParentStepID:    nil,
		CommitSHABefore: "abc123",
		CommitSHAAfter:  "",
		AgentConfig: AgentConfig{
			Model:        "test-model",
			MaxTokens:    1000,
			Temperature:  0.7,
			SystemPrompt: "test prompt",
		},
		StartTime:  time.Now(),
		EndTime:    nil,
		DurationMs: nil,
		TokenUsage: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Cost:             0.001,
		},
		ExitCode:  nil,
		ProjectID: "test-project",
		CreatedAt: time.Now(),
	}

	err := sdb.CreateStep(parentStep)
	if err != nil {
		t.Fatalf("Failed to create parent step: %v", err)
	}

	// Create child step
	childStep := &Step{
		Active:          true,
		ParentStepID:    &parentStep.ID,
		CommitSHABefore: "def456",
		CommitSHAAfter:  "",
		AgentConfig: AgentConfig{
			Model:        "test-model",
			MaxTokens:    1000,
			Temperature:  0.7,
			SystemPrompt: "test prompt",
		},
		StartTime:  time.Now(),
		EndTime:    nil,
		DurationMs: nil,
		TokenUsage: TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Cost:             0.001,
		},
		ExitCode:  nil,
		ProjectID: "test-project",
		CreatedAt: time.Now(),
	}

	err = sdb.CreateStep(childStep)
	if err != nil {
		t.Fatalf("Failed to create child step: %v", err)
	}

	// Retrieve child step and verify parent relationship
	retrievedChild, err := sdb.GetStep(childStep.ID)
	if err != nil {
		t.Fatalf("Failed to get child step: %v", err)
	}

	if retrievedChild.ParentStepID == nil {
		t.Fatal("Child step should have parent step ID")
	}

	if *retrievedChild.ParentStepID != parentStep.ID {
		t.Errorf("Parent step ID mismatch: got %d, want %d", *retrievedChild.ParentStepID, parentStep.ID)
	}
}
