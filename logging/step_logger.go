package logging

import (
	"fmt"
	"time"
)

// StepLogger provides specialized logging for LaForge step execution
type StepLogger struct {
	logger        *Logger
	stepID        string
	projectID     string
	startTime     time.Time
	stepStartTime time.Time
}

// NewStepLogger creates a new step logger
func NewStepLogger(logger *Logger, projectID, stepID string) *StepLogger {
	now := time.Now()
	sl := &StepLogger{
		logger:        logger,
		stepID:        stepID,
		projectID:     projectID,
		startTime:     now,
		stepStartTime: now,
	}

	// Set context on the underlying logger
	logger.SetProjectID(projectID)
	logger.SetStepID(stepID)

	return sl
}

// LogStepStart logs the beginning of a step
func (sl *StepLogger) LogStepStart(projectID string) {
	sl.logger.Info("step", fmt.Sprintf("Starting LaForge step for project '%s'", projectID), map[string]interface{}{
		"project_id": projectID,
		"step_id":    sl.stepID,
		"timestamp":  sl.stepStartTime.Format(time.RFC3339),
	})
}

// LogStepEnd logs the completion of a step
func (sl *StepLogger) LogStepEnd(success bool, exitCode int) {
	status := "completed"
	if !success {
		status = "failed"
	}

	sl.logger.Info("step", fmt.Sprintf("LaForge step %s", status), map[string]interface{}{
		"project_id": sl.projectID,
		"step_id":    sl.stepID,
		"status":     status,
		"exit_code":  exitCode,
		"timestamp":  time.Now().Format(time.RFC3339),
	})
}

// LogWorktreeCreation logs worktree creation
func (sl *StepLogger) LogWorktreeCreation(worktreePath string, branch string) {
	sl.logger.Info("git", "Creating temporary git worktree", map[string]interface{}{
		"worktree_path": worktreePath,
		"branch":        branch,
		"project_id":    sl.projectID,
		"step_id":       sl.stepID,
	})
}

// LogWorktreeCleanup logs worktree cleanup
func (sl *StepLogger) LogWorktreeCleanup(worktreePath string) {
	sl.logger.Info("git", "Cleaning up temporary worktree", map[string]interface{}{
		"worktree_path": worktreePath,
		"project_id":    sl.projectID,
		"step_id":       sl.stepID,
	})
}

// LogDockerClientInit logs Docker client initialization
func (sl *StepLogger) LogDockerClientInit() {
	sl.logger.Info("docker", "Initializing Docker client", map[string]interface{}{
		"project_id": sl.projectID,
		"step_id":    sl.stepID,
	})
}

// LogContainerLaunch logs container launch
func (sl *StepLogger) LogContainerLaunch(image string, containerName string, config map[string]interface{}) {
	sl.logger.Info("docker", fmt.Sprintf("Launching agent container with image '%s'", image), map[string]interface{}{
		"image":          image,
		"container_name": containerName,
		"config":         config,
		"project_id":     sl.projectID,
		"step_id":        sl.stepID,
	})
}

// LogContainerCompletion logs container completion
func (sl *StepLogger) LogContainerCompletion(exitCode int64, logs string) {
	sl.logger.Info("docker", fmt.Sprintf("Agent container completed with exit code: %d", exitCode), map[string]interface{}{
		"exit_code":  exitCode,
		"has_logs":   len(logs) > 0,
		"log_length": len(logs),
		"project_id": sl.projectID,
		"step_id":    sl.stepID,
	})

	// Log container logs if present
	if len(logs) > 0 {
		sl.logger.Debug("docker", "Container logs", map[string]interface{}{
			"logs":       logs,
			"project_id": sl.projectID,
			"step_id":    sl.stepID,
		})
	}
}

// LogGitChanges logs git change detection
func (sl *StepLogger) LogGitChanges(hasChanges bool, repoPath string) {
	action := "No changes detected"
	if hasChanges {
		action = "Changes detected"
	}

	sl.logger.Info("git", action, map[string]interface{}{
		"has_changes": hasChanges,
		"repo_path":   repoPath,
		"project_id":  sl.projectID,
		"step_id":     sl.stepID,
	})
}

// LogGitCommit logs git commit
func (sl *StepLogger) LogGitCommit(message string, repoPath string) {
	sl.logger.Info("git", "Committing changes to git", map[string]interface{}{
		"message":    message,
		"repo_path":  repoPath,
		"project_id": sl.projectID,
		"step_id":    sl.stepID,
	})
}

// LogError logs an error that occurred during step execution
func (sl *StepLogger) LogError(component, message string, err error, metadata map[string]interface{}) {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["project_id"] = sl.projectID
	metadata["step_id"] = sl.stepID

	sl.logger.Error(component, message, err, metadata)
}

// LogWarning logs a warning during step execution
func (sl *StepLogger) LogWarning(component, message string, metadata map[string]interface{}) {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["project_id"] = sl.projectID
	metadata["step_id"] = sl.stepID

	sl.logger.Warn(component, message, metadata)
}

// LogStepPhase logs the start of a major phase in step execution
func (sl *StepLogger) LogStepPhase(phase string, description string) {
	sl.logger.Info("step", fmt.Sprintf("Starting phase: %s", phase), map[string]interface{}{
		"phase":       phase,
		"description": description,
		"project_id":  sl.projectID,
		"step_id":     sl.stepID,
	})
}

// LogResourceUsage logs resource usage information
func (sl *StepLogger) LogResourceUsage(resourceType string, usage map[string]interface{}) {
	sl.logger.Info("resources", fmt.Sprintf("Resource usage: %s", resourceType), map[string]interface{}{
		"resource_type": resourceType,
		"usage":         usage,
		"project_id":    sl.projectID,
		"step_id":       sl.stepID,
	})
}

// GenerateStepID generates a unique step ID based on timestamp
func GenerateStepID() string {
	return fmt.Sprintf("S%d", time.Now().Unix())
}
