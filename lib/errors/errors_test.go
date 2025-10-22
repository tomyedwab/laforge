package errors

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestNewError(t *testing.T) {
	err := New(ErrInvalidInput, "invalid input provided")

	if err.Type != ErrInvalidInput {
		t.Errorf("Expected error type %v, got %v", ErrInvalidInput, err.Type)
	}

	if err.Message != "invalid input provided" {
		t.Errorf("Expected message 'invalid input provided', got '%s'", err.Message)
	}

	if err.Cause != nil {
		t.Errorf("Expected no cause, got %v", err.Cause)
	}

	if err.Error() != "invalid input provided" {
		t.Errorf("Expected error string 'invalid input provided', got '%s'", err.Error())
	}
}

func TestNewfError(t *testing.T) {
	err := Newf(ErrNotFound, "resource '%s' not found", "test-resource")

	if err.Type != ErrNotFound {
		t.Errorf("Expected error type %v, got %v", ErrNotFound, err.Type)
	}

	expectedMsg := "resource 'test-resource' not found"
	if err.Message != expectedMsg {
		t.Errorf("Expected message '%s', got '%s'", expectedMsg, err.Message)
	}
}

func TestWrapError(t *testing.T) {
	cause := errors.New("underlying error")
	err := Wrap(ErrDatabaseConnectionFailed, cause, "failed to connect to database")

	if err.Type != ErrDatabaseConnectionFailed {
		t.Errorf("Expected error type %v, got %v", ErrDatabaseConnectionFailed, err.Type)
	}

	if err.Cause != cause {
		t.Errorf("Expected cause %v, got %v", cause, err.Cause)
	}

	expectedMsg := "failed to connect to database: underlying error"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error string '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestErrorIs(t *testing.T) {
	err1 := New(ErrInvalidInput, "invalid input")
	err2 := New(ErrInvalidInput, "different message")
	err3 := New(ErrNotFound, "not found")

	if !err1.Is(err2) {
		t.Error("Expected err1.Is(err2) to be true (same type)")
	}

	if err1.Is(err3) {
		t.Error("Expected err1.Is(err3) to be false (different type)")
	}
}

func TestErrorUnwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := Wrap(ErrDatabaseConnectionFailed, cause, "database error")

	unwrapped := err.Unwrap()
	if unwrapped != cause {
		t.Errorf("Expected unwrapped error to be %v, got %v", cause, unwrapped)
	}
}

func TestErrorWithContext(t *testing.T) {
	err := New(ErrContainerExecutionFailed, "container failed")
	err.WithContext("container_id", "abc123")
	err.WithContext("exit_code", 1)

	containerID, ok := err.GetContext("container_id")
	if !ok || containerID != "abc123" {
		t.Errorf("Expected container_id context to be 'abc123', got %v", containerID)
	}

	exitCode, ok := err.GetContext("exit_code")
	if !ok || exitCode != 1 {
		t.Errorf("Expected exit_code context to be 1, got %v", exitCode)
	}

	_, ok = err.GetContext("nonexistent")
	if ok {
		t.Error("Expected GetContext('nonexistent') to return false")
	}
}

func TestIsErrorType(t *testing.T) {
	laforgeErr := New(ErrInvalidInput, "invalid input")
	regularErr := errors.New("regular error")

	if !IsErrorType(laforgeErr, ErrInvalidInput) {
		t.Error("Expected IsErrorType to return true for LaForgeError")
	}

	if IsErrorType(regularErr, ErrInvalidInput) {
		t.Error("Expected IsErrorType to return false for regular error")
	}
}

func TestGetErrorType(t *testing.T) {
	laforgeErr := New(ErrDockerNotAvailable, "docker not available")
	regularErr := errors.New("regular error")

	if GetErrorType(laforgeErr) != ErrDockerNotAvailable {
		t.Error("Expected GetErrorType to return correct error type for LaForgeError")
	}

	if GetErrorType(regularErr) != ErrUnknown {
		t.Error("Expected GetErrorType to return ErrUnknown for regular error")
	}

	if GetErrorType(nil) != ErrUnknown {
		t.Error("Expected GetErrorType to return ErrUnknown for nil error")
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		err      error
		expected int
	}{
		{nil, 0},
		{New(ErrInvalidInput, "invalid input"), 1},
		{New(ErrProjectNotFound, "project not found"), 2},
		{New(ErrProjectAlreadyExists, "project exists"), 3},
		{New(ErrPermissionDenied, "permission denied"), 4},
		{New(ErrTimeout, "timeout"), 5},
		{New(ErrGitNotAvailable, "git not available"), 10},
		{New(ErrDatabaseConnectionFailed, "db connection failed"), 20},
		{New(ErrDockerNotAvailable, "docker not available"), 30},
		{New(ErrTaskDependencyNotMet, "dependency not met"), 40},
		{New(ErrUnknown, "unknown error"), 1},
		{errors.New("regular error"), 1},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.err), func(t *testing.T) {
			got := ExitCode(tt.err)
			if got != tt.expected {
				t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.expected)
			}
		})
	}
}

func TestUserFriendlyMessage(t *testing.T) {
	tests := []struct {
		err      error
		expected string
	}{
		{nil, ""},
		{New(ErrInvalidInput, "invalid input"), "Invalid input provided. Please check your command and try again."},
		{New(ErrProjectNotFound, "project not found"), "Project not found. Please check the project ID and try again."},
		{New(ErrDockerNotAvailable, "docker not available"), "Docker is not available. Please install Docker and try again."},
		{New(ErrTaskDependencyNotMet, "dependency not met"), "Task dependencies are not met. Please complete upstream tasks first."},
		{errors.New("regular error"), "regular error"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.err), func(t *testing.T) {
			got := UserFriendlyMessage(tt.err)
			if got != tt.expected {
				t.Errorf("UserFriendlyMessage(%v) = %q, want %q", tt.err, got, tt.expected)
			}
		})
	}
}

func TestSuggestion(t *testing.T) {
	tests := []struct {
		err      error
		expected string
	}{
		{nil, ""},
		{New(ErrInvalidInput, "invalid input"), "Use 'laforge --help' to see available commands and options."},
		{New(ErrProjectNotFound, "project not found"), "Use 'laforge init <project-id>' to create a new project."},
		{New(ErrDockerNotAvailable, "docker not available"), "Install Docker from https://docs.docker.com/get-docker/"},
		{New(ErrTaskDependencyNotMet, "dependency not met"), "Use 'latasks list' to see task dependencies and their status."},
		{errors.New("regular error"), "Check the error message and logs for more information."},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.err), func(t *testing.T) {
			got := Suggestion(tt.err)
			if got != tt.expected {
				t.Errorf("Suggestion(%v) = %q, want %q", tt.err, got, tt.expected)
			}
		})
	}
}

func TestCommonErrorConstructors(t *testing.T) {
	// Test NewInvalidInputError
	err := NewInvalidInputError("invalid input")
	if err.Type != ErrInvalidInput {
		t.Errorf("Expected error type %v, got %v", ErrInvalidInput, err.Type)
	}

	// Test NewNotFoundError
	err = NewNotFoundError("project", "test-project")
	if err.Type != ErrNotFound {
		t.Errorf("Expected error type %v, got %v", ErrNotFound, err.Type)
	}
	if !strings.Contains(err.Message, "test-project") {
		t.Errorf("Expected message to contain 'test-project', got '%s'", err.Message)
	}

	// Test NewAlreadyExistsError
	err = NewAlreadyExistsError("project", "test-project")
	if err.Type != ErrAlreadyExists {
		t.Errorf("Expected error type %v, got %v", ErrAlreadyExists, err.Type)
	}

	// Test NewProjectNotFoundError
	err = NewProjectNotFoundError("test-project")
	if err.Type != ErrProjectNotFound {
		t.Errorf("Expected error type %v, got %v", ErrProjectNotFound, err.Type)
	}

	// Test NewContainerExecutionError
	err = NewContainerExecutionError(errors.New("container failed"), "abc123")
	if err.Type != ErrContainerExecutionFailed {
		t.Errorf("Expected error type %v, got %v", ErrContainerExecutionFailed, err.Type)
	}

	containerID, ok := err.GetContext("container_id")
	if !ok || containerID != "abc123" {
		t.Errorf("Expected container_id context to be 'abc123', got %v", containerID)
	}
}
