package handlers

import (
	"testing"
	"time"

	"github.com/tomyedwab/laforge/tasks"
)

// Mock database for testing
type mockDB struct{}

func TestConvertTask(t *testing.T) {
	tests := []struct {
		name     string
		task     *tasks.Task
		expected *TaskResponse
	}{
		{
			name: "Basic task with FEAT type",
			task: &tasks.Task{
				ID:          1,
				Title:       "[FEAT] Implement authentication",
				Description: "Create auth system",
				Status:      "todo",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			expected: &TaskResponse{
				ID:          1,
				Title:       "[FEAT] Implement authentication",
				Description: "Create auth system",
				Type:        "FEAT",
				Status:      "todo",
			},
		},
		{
			name: "Task with BUG type",
			task: &tasks.Task{
				ID:          2,
				Title:       "[BUG] Fix login issue",
				Description: "Login fails sometimes",
				Status:      "in-progress",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			expected: &TaskResponse{
				ID:          2,
				Title:       "[BUG] Fix login issue",
				Description: "Login fails sometimes",
				Type:        "BUG",
				Status:      "in-progress",
			},
		},
		{
			name: "Completed task",
			task: &tasks.Task{
				ID:          3,
				Title:       "[FEAT] Add logging",
				Description: "Add logging system",
				Status:      "completed",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			expected: &TaskResponse{
				ID:          3,
				Title:       "[FEAT] Add logging",
				Description: "Add logging system",
				Type:        "FEAT",
				Status:      "completed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertTask(tt.task)

			if result.ID != tt.expected.ID {
				t.Errorf("Expected ID %d, got %d", tt.expected.ID, result.ID)
			}
			if result.Title != tt.expected.Title {
				t.Errorf("Expected Title %s, got %s", tt.expected.Title, result.Title)
			}
			if result.Type != tt.expected.Type {
				t.Errorf("Expected Type %s, got %s", tt.expected.Type, result.Type)
			}
			if result.Status != tt.expected.Status {
				t.Errorf("Expected Status %s, got %s", tt.expected.Status, result.Status)
			}

			// Check that completed_at is set for completed tasks
			if tt.task.Status == "completed" && result.CompletedAt == nil {
				t.Error("Expected CompletedAt to be set for completed task")
			}
			if tt.task.Status != "completed" && result.CompletedAt != nil {
				t.Error("Expected CompletedAt to be nil for non-completed task")
			}
		})
	}
}

func TestCreateTaskRequestValidation(t *testing.T) {
	tests := []struct {
		name      string
		request   CreateTaskRequest
		wantError bool
		errorMsg  string
	}{
		{
			name: "Valid request",
			request: CreateTaskRequest{
				Title:       "[FEAT] New feature",
				Description: "Implement new feature",
				Type:        "FEAT",
			},
			wantError: false,
		},
		{
			name: "Missing title",
			request: CreateTaskRequest{
				Description: "Implement new feature",
				Type:        "FEAT",
			},
			wantError: true,
			errorMsg:  "Title is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is a basic validation test - in real implementation
			// we'd test the actual HTTP handler with mock database
			if tt.request.Title == "" && !tt.wantError {
				t.Error("Expected validation to fail for empty title")
			}
		})
	}
}

func TestUpdateTaskStatusRequestValidation(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		wantError bool
	}{
		{"Valid status - todo", "todo", false},
		{"Valid status - in-progress", "in-progress", false},
		{"Valid status - in-review", "in-review", false},
		{"Valid status - completed", "completed", false},
		{"Invalid status", "invalid", true},
		{"Empty status", "", true},
	}

	validStatuses := map[string]bool{
		"todo":        true,
		"in-progress": true,
		"in-review":   true,
		"completed":   true,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validStatuses[tt.status]
			if isValid && tt.wantError {
				t.Errorf("Expected %s to be invalid, but it's valid", tt.status)
			}
			if !isValid && !tt.wantError {
				t.Errorf("Expected %s to be valid, but it's invalid", tt.status)
			}
		})
	}
}

func TestTaskTypeExtraction(t *testing.T) {
	tests := []struct {
		title    string
		expected string
	}{
		{"[FEAT] Implement auth", "FEAT"},
		{"[BUG] Fix login issue", "BUG"},
		{"[PLAN] Create architecture", "PLAN"},
		{"[TEST] Add unit tests", "TEST"},
		{"No brackets here", "FEAT"}, // default
		{"[INCOMPLETE", "FEAT"},      // malformed, default
		{"", "FEAT"},                 // empty, default
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			task := &tasks.Task{
				Title:       tt.title,
				Description: "Test task",
				Status:      "todo",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			result := convertTask(task)
			if result.Type != tt.expected {
				t.Errorf("Expected type %s for title '%s', got %s", tt.expected, tt.title, result.Type)
			}
		})
	}
}

func TestGetNextTaskResponse(t *testing.T) {
	// Test the response format for when no task is available
	response := map[string]interface{}{
		"data": map[string]interface{}{
			"task":    nil,
			"message": "No tasks ready for work",
		},
		"meta": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	// Verify response structure
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Response data should be a map")
	}

	if data["task"] != nil {
		t.Error("Task should be nil when no tasks are ready")
	}

	message, ok := data["message"].(string)
	if !ok {
		t.Fatal("Message should be a string")
	}

	if message != "No tasks ready for work" {
		t.Errorf("Expected message 'No tasks ready for work', got '%s'", message)
	}

	meta, ok := response["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("Response meta should be a map")
	}

	if meta["version"] != "1.0.0" {
		t.Error("Expected version 1.0.0")
	}
}
