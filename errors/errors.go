package errors

import (
	"errors"
	"fmt"
)

// ErrorType represents different types of errors that can occur in LaForge
type ErrorType int

const (
	// General errors
	ErrUnknown ErrorType = iota
	ErrInvalidInput
	ErrNotFound
	ErrAlreadyExists
	ErrPermissionDenied
	ErrTimeout

	// Project errors
	ErrProjectNotFound
	ErrProjectAlreadyExists
	ErrInvalidProjectID

	// Git errors
	ErrGitNotAvailable
	ErrGitRepositoryNotFound
	ErrGitOperationFailed

	// Database errors
	ErrDatabaseNotFound
	ErrDatabaseConnectionFailed
	ErrDatabaseCorrupted
	ErrDatabaseOperationFailed

	// Docker errors
	ErrDockerNotAvailable
	ErrDockerConnectionFailed
	ErrContainerCreationFailed
	ErrContainerStartFailed
	ErrContainerExecutionFailed

	// Task errors
	ErrTaskNotFound
	ErrInvalidTaskStatus
	ErrTaskDependencyNotMet
	ErrTaskReviewRequired
)

// LaForgeError represents a LaForge-specific error with additional context
type LaForgeError struct {
	Type    ErrorType
	Message string
	Cause   error
	Context map[string]interface{}
}

// Error implements the error interface
func (e *LaForgeError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap returns the cause of the error
func (e *LaForgeError) Unwrap() error {
	return e.Cause
}

// Is checks if the error is of a specific type
func (e *LaForgeError) Is(target error) bool {
	targetErr, ok := target.(*LaForgeError)
	if !ok {
		return false
	}
	return e.Type == targetErr.Type
}

// WithContext adds context to the error
func (e *LaForgeError) WithContext(key string, value interface{}) *LaForgeError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// GetContext retrieves context value
func (e *LaForgeError) GetContext(key string) (interface{}, bool) {
	if e.Context == nil {
		return nil, false
	}
	val, ok := e.Context[key]
	return val, ok
}

// New creates a new LaForgeError
func New(errType ErrorType, message string) *LaForgeError {
	return &LaForgeError{
		Type:    errType,
		Message: message,
	}
}

// Newf creates a new LaForgeError with formatted message
func Newf(errType ErrorType, format string, args ...interface{}) *LaForgeError {
	return &LaForgeError{
		Type:    errType,
		Message: fmt.Sprintf(format, args...),
	}
}

// Wrap wraps an existing error with LaForgeError context
func Wrap(errType ErrorType, cause error, message string) *LaForgeError {
	return &LaForgeError{
		Type:    errType,
		Message: message,
		Cause:   cause,
	}
}

// Wrapf wraps an existing error with LaForgeError context and formatted message
func Wrapf(errType ErrorType, cause error, format string, args ...interface{}) *LaForgeError {
	return &LaForgeError{
		Type:    errType,
		Message: fmt.Sprintf(format, args...),
		Cause:   cause,
	}
}

// IsErrorType checks if an error is of a specific LaForgeError type
func IsErrorType(err error, errType ErrorType) bool {
	var laforgeErr *LaForgeError
	if errors.As(err, &laforgeErr) {
		return laforgeErr.Type == errType
	}
	return false
}

// GetErrorType extracts the error type from an error
func GetErrorType(err error) ErrorType {
	var laforgeErr *LaForgeError
	if errors.As(err, &laforgeErr) {
		return laforgeErr.Type
	}
	return ErrUnknown
}

// ExitCode returns the appropriate exit code for an error
func ExitCode(err error) int {
	if err == nil {
		return 0
	}

	var laforgeErr *LaForgeError
	if errors.As(err, &laforgeErr) {
		switch laforgeErr.Type {
		case ErrInvalidInput:
			return 1
		case ErrNotFound, ErrProjectNotFound, ErrTaskNotFound, ErrDatabaseNotFound:
			return 2
		case ErrAlreadyExists, ErrProjectAlreadyExists:
			return 3
		case ErrPermissionDenied:
			return 4
		case ErrTimeout:
			return 5
		case ErrGitNotAvailable, ErrGitRepositoryNotFound, ErrGitOperationFailed:
			return 10
		case ErrDatabaseConnectionFailed, ErrDatabaseCorrupted, ErrDatabaseOperationFailed:
			return 20
		case ErrDockerNotAvailable, ErrDockerConnectionFailed, ErrContainerCreationFailed, ErrContainerStartFailed, ErrContainerExecutionFailed:
			return 30
		case ErrTaskDependencyNotMet, ErrTaskReviewRequired:
			return 40
		default:
			return 1
		}
	}

	// Default exit code for non-LaForge errors
	return 1
}

// UserFriendlyMessage returns a user-friendly error message
func UserFriendlyMessage(err error) string {
	if err == nil {
		return ""
	}

	var laforgeErr *LaForgeError
	if errors.As(err, &laforgeErr) {
		switch laforgeErr.Type {
		case ErrInvalidInput:
			return "Invalid input provided. Please check your command and try again."
		case ErrNotFound:
			return "The requested resource was not found."
		case ErrProjectNotFound:
			return "Project not found. Please check the project ID and try again."
		case ErrTaskNotFound:
			return "Task not found. Please check the task ID and try again."
		case ErrAlreadyExists:
			return "The resource already exists."
		case ErrProjectAlreadyExists:
			return "A project with this ID already exists. Please choose a different ID."
		case ErrPermissionDenied:
			return "Permission denied. Please check your permissions and try again."
		case ErrTimeout:
			return "The operation timed out. Please try again."
		case ErrGitNotAvailable:
			return "Git is not available. Please install Git and try again."
		case ErrGitRepositoryNotFound:
			return "Git repository not found. Please initialize a Git repository first."
		case ErrGitOperationFailed:
			return "Git operation failed. Please check your Git configuration and try again."
		case ErrDatabaseNotFound:
			return "Database not found. Please check the database path and try again."
		case ErrDatabaseConnectionFailed:
			return "Failed to connect to the database. Please check the database configuration."
		case ErrDatabaseCorrupted:
			return "The database appears to be corrupted. Please restore from a backup."
		case ErrDatabaseOperationFailed:
			return "Database operation failed. Please check the database logs for more information."
		case ErrDockerNotAvailable:
			return "Docker is not available. Please install Docker and try again."
		case ErrDockerConnectionFailed:
			return "Failed to connect to Docker. Please check if Docker is running."
		case ErrContainerCreationFailed:
			return "Failed to create container. Please check Docker configuration and available resources."
		case ErrContainerStartFailed:
			return "Failed to start container. Please check Docker logs for more information."
		case ErrContainerExecutionFailed:
			return "Container execution failed. Please check container logs for more information."
		case ErrTaskDependencyNotMet:
			return "Task dependencies are not met. Please complete upstream tasks first."
		case ErrTaskReviewRequired:
			return "This task requires review before it can be completed."
		default:
			return laforgeErr.Message
		}
	}

	// For non-LaForge errors, return the original error message
	return err.Error()
}

// Suggestion returns a suggestion for resolving the error
func Suggestion(err error) string {
	if err == nil {
		return ""
	}

	var laforgeErr *LaForgeError
	if errors.As(err, &laforgeErr) {
		switch laforgeErr.Type {
		case ErrInvalidInput:
			return "Use 'laforge --help' to see available commands and options."
		case ErrProjectNotFound:
			return "Use 'laforge init <project-id>' to create a new project."
		case ErrProjectAlreadyExists:
			return "Use a different project ID or delete the existing project first."
		case ErrGitNotAvailable:
			return "Install Git from https://git-scm.com/downloads"
		case ErrDockerNotAvailable:
			return "Install Docker from https://docs.docker.com/get-docker/"
		case ErrDockerConnectionFailed:
			return "Start Docker service: 'sudo systemctl start docker' or 'sudo service docker start'"
		case ErrDatabaseConnectionFailed:
			return "Check database file permissions and ensure it's not corrupted."
		case ErrTaskDependencyNotMet:
			return "Use 'latasks list' to see task dependencies and their status."
		case ErrTaskReviewRequired:
			return "Use 'latasks review' to submit the task for review."
		default:
			if laforgeErr.Cause != nil {
				return fmt.Sprintf("Underlying error: %v", laforgeErr.Cause)
			}
			return "Check the logs for more detailed error information."
		}
	}

	return "Check the error message and logs for more information."
}

// Common error constructors

// NewInvalidInputError creates an error for invalid input
func NewInvalidInputError(message string) *LaForgeError {
	return New(ErrInvalidInput, message)
}

// NewNotFoundError creates an error for not found resources
func NewNotFoundError(resource, identifier string) *LaForgeError {
	return Newf(ErrNotFound, "%s '%s' not found", resource, identifier)
}

// NewAlreadyExistsError creates an error for already existing resources
func NewAlreadyExistsError(resource, identifier string) *LaForgeError {
	return Newf(ErrAlreadyExists, "%s '%s' already exists", resource, identifier)
}

// NewProjectNotFoundError creates an error for project not found
func NewProjectNotFoundError(projectID string) *LaForgeError {
	return Newf(ErrProjectNotFound, "Project '%s' not found", projectID)
}

// NewProjectAlreadyExistsError creates an error for project already existing
func NewProjectAlreadyExistsError(projectID string) *LaForgeError {
	return Newf(ErrProjectAlreadyExists, "Project '%s' already exists", projectID)
}

// NewGitNotAvailableError creates an error for Git not being available
func NewGitNotAvailableError(cause error) *LaForgeError {
	return Wrap(ErrGitNotAvailable, cause, "Git is not available")
}

// NewDockerNotAvailableError creates an error for Docker not being available
func NewDockerNotAvailableError(cause error) *LaForgeError {
	return Wrap(ErrDockerNotAvailable, cause, "Docker is not available")
}

// NewDatabaseConnectionError creates an error for database connection failures
func NewDatabaseConnectionError(cause error) *LaForgeError {
	return Wrap(ErrDatabaseConnectionFailed, cause, "Failed to connect to database")
}

// NewContainerExecutionError creates an error for container execution failures
func NewContainerExecutionError(cause error, containerID string) *LaForgeError {
	return Wrapf(ErrContainerExecutionFailed, cause, "Container %s execution failed", containerID).
		WithContext("container_id", containerID)
}
