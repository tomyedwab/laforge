package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tomyedwab/laforge/database"
	"github.com/tomyedwab/laforge/docker"
	"github.com/tomyedwab/laforge/errors"
	"github.com/tomyedwab/laforge/git"
	"github.com/tomyedwab/laforge/logging"
	"github.com/tomyedwab/laforge/projects"
	"github.com/tomyedwab/laforge/steps"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "laforge",
	Short: "LaForge - A long-running collaborative coding agent",
	Long: `LaForge is an experimental coding agent that can run for long periods of time
semi-autonomously. It provides tools for managing tasks, running steps, and
collaborating with human reviewers through artifacts and feedback.`,
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
}

func init() {
	// Add persistent flags that will be available to all commands
	rootCmd.PersistentFlags().String("config", "", "config file (default is $HOME/.laforge.yaml)")
	rootCmd.PersistentFlags().Bool("verbose", false, "enable verbose output")
	rootCmd.PersistentFlags().Bool("quiet", false, "suppress non-error output")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Use LaForge error handling for better error messages
		fmt.Fprintf(os.Stderr, "Error: %s\n", errors.UserFriendlyMessage(err))

		// Add suggestion if available
		if suggestion := errors.Suggestion(err); suggestion != "" {
			fmt.Fprintf(os.Stderr, "Suggestion: %s\n", suggestion)
		}

		os.Exit(errors.ExitCode(err))
	}
}

func init() {
	// Add subcommands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(stepCmd)
	rootCmd.AddCommand(stepsCmd)
	rootCmd.AddCommand(stepInfoCmd)

	// Add step subcommands
	stepCmd.AddCommand(stepRollbackCmd)
}

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init [project-id]",
	Short: "Initialize a new LaForge project",
	Long: `Initialize a new LaForge project with the specified ID.

This command creates the project directory structure, creates a task database,
and generates the project configuration file.`,
	Args: cobra.ExactArgs(1),
	RunE: runInit,
}

// stepCmd represents the step command
var stepCmd = &cobra.Command{
	Use:   "step [project-id]",
	Short: "Run a single LaForge step cycle",
	Long: `Run a single step in the LaForge process.

This command creates a temporary git worktree, copies the task database for isolation,
launches an agent container with proper mounts, captures and commits changes, and
cleans up resources.`,
	Args: cobra.ExactArgs(1),
	RunE: runStep,
}

// stepsCmd represents the steps command
var stepsCmd = &cobra.Command{
	Use:   "steps [project-id]",
	Short: "List all steps for a project",
	Long: `List all steps for a LaForge project.

This command displays a formatted list of all steps recorded for the specified project,
including step ID, status, duration, and commit information.

Examples:
  laforge steps my-project
  laforge steps my-project | grep -E "S[0-9]+" | head -10

Output includes:
  - Step ID (S1, S2, etc.)
  - Active status (active/inactive)
  - Start and end times
  - Duration in milliseconds
  - Git commit SHAs before and after
  - Exit code and token usage summary`,
	Args: cobra.ExactArgs(1),
	RunE: runSteps,
}

// stepInfoCmd represents the step info command
var stepInfoCmd = &cobra.Command{
	Use:   "info [project-id] [step-id]",
	Short: "Show detailed information about a specific step",
	Long: `Show detailed information about a specific step.

This command displays comprehensive information about a step, including timing,
commit SHAs, agent configuration, token usage, and exit status.

Examples:
  laforge step info my-project S1
  laforge step info my-project 5

Information displayed:
  - Step ID and active status
  - Parent step ID (for tracking execution history)
  - Start and end times with duration
  - Git commit SHAs before and after execution
  - Complete agent configuration (model, temperature, tools, etc.)
  - Token usage statistics (prompt, completion, total, cost)
  - Exit code and any error information
  - Project ID and creation timestamp`,
	Args: cobra.ExactArgs(2),
	RunE: runStepInfo,
}

// stepRollbackCmd represents the step rollback command
var stepRollbackCmd = &cobra.Command{
	Use:   "rollback [project-id] [step-id]",
	Short: "Rollback to a previous step",
	Long: `Rollback to a previous step by deactivating subsequent steps and reverting git changes.

This command allows you to revert your project to the state at a specific step by:
1. Deactivating all steps after the target step in the database
2. Resetting the git repository to the commit before the target step

Examples:
  laforge step rollback my-project S3    # Rollback to step 3
  laforge step rollback my-project 1     # Rollback to step 1

Safety features:
  - User confirmation required before rollback
  - Git repository must be clean (no uncommitted changes)
  - All steps >= target step ID are deactivated
  - Git reset to commit before target step

Use with caution as this will permanently deactivate subsequent steps and discard changes.

Common use cases:
  - Revert to a known good state after problematic changes
  - Explore alternative approaches by rolling back and trying again
  - Recover from agent errors or unexpected behavior`,
	Args: cobra.ExactArgs(2),
	RunE: runStepRollback,
}

func init() {
	// Add flags for init command
	initCmd.Flags().String("name", "", "project name")
	initCmd.Flags().String("description", "", "project description")
	initCmd.Flags().String("agent-image", "", "default Docker image for the agent container")
	initCmd.Flags().String("agent-config-file", "", "path to custom agents.yml configuration file")

	// Add flags for step command
	stepCmd.Flags().String("agent-config", "", "agent configuration name from agents.yml (overrides --agent-image)")
	stepCmd.Flags().Duration("timeout", 0, "timeout for step execution (0 means no timeout)")
}

// runInit is the handler for the init command
func runInit(cmd *cobra.Command, args []string) error {
	projectID := args[0]

	// Validate project ID
	if projectID == "" {
		return errors.NewInvalidInputError("project ID cannot be empty")
	}

	// Get flags
	name, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("description")
	agentImage, _ := cmd.Flags().GetString("agent-image")
	agentConfigFile, _ := cmd.Flags().GetString("agent-config-file")

	// Use project ID as name if name is not provided
	if name == "" {
		name = projectID
	}

	// Create the project
	project, err := projects.CreateProject(projectID, name, description)
	if err != nil {
		return errors.Wrap(errors.ErrProjectAlreadyExists, err, "failed to create project")
	}

	// Handle custom agent configuration if provided
	if agentConfigFile != "" {
		// Load custom configuration from file
		if err := projects.LoadCustomAgentsConfig(projectID, agentConfigFile); err != nil {
			return errors.Wrap(errors.ErrUnknown, err, "failed to load custom agents configuration")
		}
	} else if agentImage != "" {
		// Update default agent image if specified
		if err := projects.UpdateDefaultAgentImage(projectID, agentImage); err != nil {
			return errors.Wrap(errors.ErrUnknown, err, "failed to update default agent image")
		}
	}

	// Get project directory for display
	projectDir, err := projects.GetProjectDir(projectID)
	if err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to get project directory")
	}

	fmt.Printf("Successfully created LaForge project '%s'\n", project.ID)
	if project.Name != project.ID {
		fmt.Printf("Project name: %s\n", project.Name)
	}
	if project.Description != "" {
		fmt.Printf("Description: %s\n", project.Description)
	}
	fmt.Printf("Location: %s\n", projectDir)

	return nil
}

// runStep is the handler for the step command
func runStep(cmd *cobra.Command, args []string) error {
	projectID := args[0]

	// Validate project ID
	if projectID == "" {
		return errors.NewInvalidInputError("project ID cannot be empty")
	}

	// Get flags
	agentConfigName, _ := cmd.Flags().GetString("agent-config")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	verbose, _ := cmd.Flags().GetBool("verbose")
	quiet, _ := cmd.Flags().GetBool("quiet")

	sourceDir, _ := os.Getwd()

	// Check if project exists
	exists, err := projects.ProjectExists(projectID)
	if err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to check if project exists")
	}
	if !exists {
		return errors.NewProjectNotFoundError(projectID)
	}

	// Get task database path
	taskDBPath, err := projects.GetProjectTaskDatabase(projectID)
	if err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to get project task database path")
	}

	// Load agents configuration from file
	agentsConfig, err := projects.LoadAgentsConfig(projectID)
	if err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to load agents configuration")
	}

	var agentConfig *projects.AgentConfig
	if agentConfigName != "" {
		// Get the specified agent configuration
		agent, exists := agentsConfig.GetAgent(agentConfigName)
		if !exists {
			return errors.NewInvalidInputError(fmt.Sprintf("agent configuration '%s' not found", agentConfigName))
		}
		agentConfig = &agent
	} else {
		// Get the specified agent configuration
		agent, exists := agentsConfig.GetDefaultAgent()
		if !exists {
			return errors.NewInvalidInputError(fmt.Sprintf("agent configuration '%s' not found", agentConfigName))
		}
		agentConfig = &agent
	}

	// Override timeout if provided via flag
	if timeout > 0 {
		agentConfig.Runtime.Timeout = timeout.String()
	}

	// Set up logging
	logger := logging.GetLogger()
	if verbose {
		// Set debug level for verbose output
		logger = logging.NewLogger(logging.DEBUG, "")
	} else if quiet {
		// Set warn level for quiet output
		logger = logging.NewLogger(logging.WARN, "")
	}

	// Open step database for recording
	stepDB, err := projects.OpenProjectStepDatabase(projectID)
	if err != nil {
		return errors.Wrap(errors.ErrDatabaseConnectionFailed, err, "failed to open project step database")
	}
	defer stepDB.Close()

	// Get the next step ID from database
	stepID, err := stepDB.GetNextStepID()
	if err != nil {
		return errors.Wrap(errors.ErrDatabaseOperationFailed, err, "failed to get next step ID")
	}

	// Get current commit SHA before step execution
	commitSHABefore, err := git.GetCurrentCommitSHA(sourceDir)
	if err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to get current commit SHA")
	}

	// Create step record in database
	step := &steps.Step{
		Active:          true,
		ParentStepID:    nil, // Will be set if this is a child step
		CommitSHABefore: commitSHABefore,
		CommitSHAAfter:  "", // Will be updated after step completion
		AgentConfig: steps.AgentConfig{
			Model:        "default", // Basic model info since projects.AgentConfig doesn't have these fields
			MaxTokens:    0,
			Temperature:  0.7,
			SystemPrompt: "", // Will be populated from agent config if available
			Tools:        []string{},
			Metadata: map[string]string{
				"agent_name": agentConfig.Name,
				"image":      agentConfig.Image,
			},
		},
		StartTime:  time.Now(),
		EndTime:    nil, // Will be updated after step completion
		DurationMs: nil, // Will be updated after step completion
		TokenUsage: steps.TokenUsage{
			PromptTokens:     0, // Will be updated after step completion
			CompletionTokens: 0, // Will be updated after step completion
			TotalTokens:      0, // Will be updated after step completion
			Cost:             0, // Will be updated after step completion
		},
		ExitCode:  nil, // Will be updated after step completion
		ProjectID: projectID,
		CreatedAt: time.Now(),
	}

	if err := stepDB.CreateStep(step); err != nil {
		return errors.Wrap(errors.ErrDatabaseOperationFailed, err, "failed to create step record")
	}

	// Use database-generated step ID instead of timestamp-based ID
	dbStepID := fmt.Sprintf("S%d", step.ID)
	stepLogger := logging.NewStepLogger(logger, projectID, dbStepID)

	// Log step start
	stepLogger.LogStepStart(projectID)
	stepStartTime := time.Now()

	// Declare worktree variable for use in defer
	var worktree *git.Worktree

	defer func() {
		// Calculate duration and update step record
		duration := time.Since(stepStartTime)
		durationMs := int(duration.Milliseconds())
		endTime := time.Now()

		// Update step record with completion data
		exitCode := int64(0)
		if err != nil {
			exitCode = 1
		}

		// Get commit SHA after step execution (if there were changes and worktree exists)
		commitSHAAfter := commitSHABefore
		if worktree != nil {
			if hasChanges, _ := hasGitChanges(worktree.Path); hasChanges {
				if sha, err := git.GetCurrentCommitSHA(worktree.Path); err == nil {
					commitSHAAfter = sha
				}
			}
		}

		if updateErr := stepDB.UpdateStep(step.ID, commitSHAAfter, endTime, durationMs, int(exitCode), step.TokenUsage); updateErr != nil {
			stepLogger.LogError("database", "Failed to update step record", updateErr, map[string]interface{}{
				"step_id": step.ID,
			})
		}

		// Log step completion
		stepLogger.LogStepEnd(err == nil, duration, exitCode)
	}()

	// Step 1: Create temporary git worktree
	stepLogger.LogStepPhase("worktree", "Creating temporary git worktree")
	worktree, err = git.CreateTempWorktreeWithStep(sourceDir, stepID)
	if err != nil {
		stepLogger.LogError("git", "Failed to create temporary worktree", err, map[string]interface{}{
			"source_dir": sourceDir,
		})
		return errors.Wrap(errors.ErrUnknown, err, "failed to create temporary worktree")
	}
	stepLogger.LogWorktreeCreation(worktree.Path, fmt.Sprintf("step-S%d", stepID))
	defer func() {
		// Clean up worktree
		stepLogger.LogWorktreeCleanup(worktree.Path)
		if cleanupErr := git.RemoveWorktree(worktree); cleanupErr != nil {
			stepLogger.LogWarning("git", "Failed to remove worktree", map[string]interface{}{
				"error": cleanupErr.Error(),
			})
		}
	}()

	// Step 2: Copy task database for isolation
	stepLogger.LogStepPhase("database", "Copying task database for isolation")
	tempDBPath, err := database.CreateTempDatabaseCopy(taskDBPath, "step")
	if err != nil {
		stepLogger.LogError("database", "Failed to create temporary database copy", err, map[string]interface{}{
			"source_path": taskDBPath,
		})
		return errors.Wrap(errors.ErrUnknown, err, "failed to create temporary database copy")
	}
	stepLogger.LogDatabaseCopy(taskDBPath, tempDBPath)
	defer func() {
		// Clean up temporary database
		stepLogger.LogDatabaseCleanup(tempDBPath)
		if cleanupErr := database.CleanupTempDatabase(tempDBPath); cleanupErr != nil {
			stepLogger.LogWarning("database", "Failed to cleanup temporary database", map[string]interface{}{
				"error": cleanupErr.Error(),
			})
		}
	}()

	// Step 3: Create Docker client
	stepLogger.LogStepPhase("docker", "Initializing Docker client")
	dockerClient, err := docker.NewClient()
	if err != nil {
		stepLogger.LogError("docker", "Failed to create Docker client", err, nil)
		return errors.Wrap(errors.ErrUnknown, err, "failed to create Docker client")
	}
	stepLogger.LogDockerClientInit()
	defer dockerClient.Close()

	// Step 4: Create log file for streaming container output
	stepLogger.LogStepPhase("logs", "Setting up log file")
	projectDir, err := projects.GetProjectDir(projectID)
	if err != nil {
		stepLogger.LogError("logs", "Failed to get project directory", err, nil)
		return errors.Wrap(errors.ErrUnknown, err, "failed to get project directory")
	}

	// Create logs directory if it doesn't exist
	logsDir := filepath.Join(projectDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		stepLogger.LogError("logs", "Failed to create logs directory", err, map[string]interface{}{
			"logs_dir": logsDir,
		})
		return errors.Wrap(errors.ErrUnknown, err, "failed to create logs directory")
	}

	// Create log file with step ID and timestamp
	logFileName := fmt.Sprintf("step-%s-%s.log", dbStepID, time.Now().Format("20060102-150405"))
	logFilePath := filepath.Join(logsDir, logFileName)
	logFile, err := os.Create(logFilePath)
	if err != nil {
		stepLogger.LogError("logs", "Failed to create log file", err, map[string]interface{}{
			"log_file_path": logFilePath,
		})
		return errors.Wrap(errors.ErrUnknown, err, "failed to create log file")
	}
	defer logFile.Close()

	logger.Info("logs", "Log file created", map[string]interface{}{
		"log_file_path": logFilePath,
		"project_id":    projectID,
		"step_id":       stepID,
	})

	// Create multi-writer to stream to both stdout and file
	multiWriter := io.MultiWriter(os.Stdout, logFile)

	// Step 5: Launch agent container
	stepLogger.LogStepPhase("container", "Launching agent container")
	containerMetrics := &docker.ContainerMetrics{}

	var exitCode int64
	var logs string

	// Use agent configuration from agents.yml
	stepLogger.LogContainerLaunch(agentConfig.Image, fmt.Sprintf("laforge-agent-%s-%s-%d", projectID, agentConfig.Name, time.Now().Unix()), map[string]interface{}{
		"work_dir":     worktree.Path,
		"task_db_path": tempDBPath,
		"agent_config": agentConfigName,
		"timeout":      agentConfig.Runtime.Timeout,
		"log_file":     logFilePath,
	})

	// Run container using AgentConfig with streaming logs
	exitCode, logs, err = dockerClient.RunAgentContainerFromConfigWithStreamingLogs(agentConfig, worktree.Path, tempDBPath, multiWriter, containerMetrics)
	if err != nil {
		stepLogger.LogError("docker", "Failed to run agent container", err, map[string]interface{}{
			"exit_code": exitCode,
		})
		return errors.Wrap(errors.ErrUnknown, err, "failed to run agent container")
	}

	stepLogger.LogContainerCompletion(exitCode, logs)

	// Log resource usage
	stepLogger.LogResourceUsage("container", map[string]interface{}{
		"duration_ms":   containerMetrics.EndTime.Sub(containerMetrics.StartTime).Milliseconds(),
		"exit_code":     containerMetrics.ExitCode,
		"log_size":      containerMetrics.LogSize,
		"error_count":   containerMetrics.ErrorCount,
		"warning_count": containerMetrics.WarningCount,
	})

	// Update step with token usage from container metrics
	step.TokenUsage = containerMetrics.TokenUsage

	// Step 6: Check if there are changes to commit
	stepLogger.LogStepPhase("git", "Checking for changes to commit")
	hasChanges, err := hasGitChanges(worktree.Path)
	if err != nil {
		stepLogger.LogError("git", "Failed to check for git changes", err, map[string]interface{}{
			"repo_path": worktree.Path,
		})
		return errors.Wrap(errors.ErrUnknown, err, "failed to check for git changes")
	}

	stepLogger.LogGitChanges(hasChanges, worktree.Path)

	if hasChanges {
		commitMessage := fmt.Sprintf("LaForge step %s - Automated changes", dbStepID)

		// Check for COMMIT.md and use its contents if it exists
		if customMessage, err := getCommitMessageFromFile(worktree.Path); err != nil {
			stepLogger.LogWarning("git", "Failed to read COMMIT.md", map[string]interface{}{
				"error": err.Error(),
			})
		} else if customMessage != "" {
			commitMessage = customMessage
		}

		stepLogger.LogGitCommit(commitMessage, worktree.Path)
		if err := commitChanges(worktree.Path, commitMessage); err != nil {
			stepLogger.LogError("git", "Failed to commit changes", err, map[string]interface{}{
				"repo_path": worktree.Path,
			})
			return errors.Wrap(errors.ErrUnknown, err, "failed to commit changes")
		}
		logger.Info("git", "Changes committed successfully", map[string]interface{}{
			"project_id": projectID,
			"step_id":    stepID,
		})
	} else {
		logger.Info("git", "No changes to commit", map[string]interface{}{
			"project_id": projectID,
			"step_id":    stepID,
		})
	}

	// Step 7: Copy changes back from temporary database to main database
	stepLogger.LogStepPhase("database", "Updating main task database")
	stepLogger.LogDatabaseUpdate(tempDBPath, taskDBPath)
	_ = os.Remove(taskDBPath)
	if err := database.CopyDatabase(tempDBPath, taskDBPath); err != nil {
		stepLogger.LogError("database", "Failed to update main task database", err, map[string]interface{}{
			"source_path": tempDBPath,
			"dest_path":   taskDBPath,
		})
		return errors.Wrap(errors.ErrUnknown, err, "failed to update main task database")
	}

	// Step 8: Automerge step branch into main branch (only if step completed successfully)
	if exitCode == 0 && hasChanges {
		stepLogger.LogStepPhase("git", "Automerging step branch into main branch")

		stepBranch := fmt.Sprintf("step-S%d", step.ID)
		mergeMessage := fmt.Sprintf("Automerge %s into main", stepBranch)

		if mergeErr := git.MergeBranch(sourceDir, stepBranch, "main", mergeMessage); mergeErr != nil {
			// Check if it's a merge conflict
			if errors.IsErrorType(mergeErr, errors.ErrGitMergeConflict) {
				// Log merge conflict but don't fail the step - keep branch for manual resolution
				stepLogger.LogWarning("git", "Merge conflict detected, keeping step branch for manual resolution", map[string]interface{}{
					"step_branch": stepBranch,
					"error":       mergeErr.Error(),
				})
				logger.Info("git", fmt.Sprintf("Merge conflict in %s, branch preserved for manual resolution", stepBranch), map[string]interface{}{
					"project_id":  projectID,
					"step_id":     stepID,
					"step_branch": stepBranch,
					"error_type":  "merge_conflict",
				})
			} else {
				// Log other merge failures but don't fail the step - keep branch for manual resolution
				stepLogger.LogWarning("git", "Automerge failed, keeping step branch for manual resolution", map[string]interface{}{
					"step_branch": stepBranch,
					"error":       mergeErr.Error(),
				})
				logger.Info("git", fmt.Sprintf("Automerge failed for %s, branch preserved for manual resolution", stepBranch), map[string]interface{}{
					"project_id":  projectID,
					"step_id":     stepID,
					"step_branch": stepBranch,
					"error":       mergeErr.Error(),
				})
			}
		} else {
			// Merge successful, delete step branch
			stepLogger.LogStepPhase("git", "Merge successful, cleaning up step branch")
			if deleteErr := git.DeleteBranch(sourceDir, stepBranch); deleteErr != nil {
				stepLogger.LogWarning("git", "Failed to delete step branch after successful merge", map[string]interface{}{
					"step_branch": stepBranch,
					"error":       deleteErr.Error(),
				})
			} else {
				logger.Info("git", fmt.Sprintf("Successfully automerged %s into main and deleted step branch", stepBranch), map[string]interface{}{
					"project_id":  projectID,
					"step_id":     stepID,
					"step_branch": stepBranch,
				})
			}
		}
	}

	logger.Info("step", fmt.Sprintf("LaForge step completed successfully for project '%s'", projectID), map[string]interface{}{
		"project_id": projectID,
		"step_id":    stepID,
		"duration":   time.Since(stepStartTime).String(),
	})

	return nil
}

// getCommitMessageFromFile checks for COMMIT.md in the repository and returns its contents
// If COMMIT.md doesn't exist, returns an empty string
func getCommitMessageFromFile(repoDir string) (string, error) {
	commitMDPath := filepath.Join(repoDir, "COMMIT.md")
	data, err := os.ReadFile(commitMDPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read COMMIT.md: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// hasGitChanges checks if there are uncommitted changes in the git repository
func hasGitChanges(repoDir string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	// If there's any output, there are changes
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// commitChanges commits all changes in the repository with the given message,
// excluding COMMIT.md if it exists
func commitChanges(repoDir string, message string) error {
	// Add all changes
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add changes: %w", err)
	}

	// Unstage COMMIT.md if it exists and was staged
	cmd = exec.Command("git", "reset", "COMMIT.md")
	cmd.Dir = repoDir
	// Ignore error if COMMIT.md doesn't exist
	_ = cmd.Run()

	// Commit changes
	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		// It's okay if there's nothing to commit
		if strings.Contains(string(output), "nothing to commit") {
			return nil
		}
		return fmt.Errorf("failed to commit changes: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// runSteps is the handler for the steps command
func runSteps(cmd *cobra.Command, args []string) error {
	projectID := args[0]

	// Validate project ID
	if projectID == "" {
		return errors.NewInvalidInputError("project ID cannot be empty")
	}

	// Check if project exists
	exists, err := projects.ProjectExists(projectID)
	if err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to check if project exists")
	}
	if !exists {
		return errors.NewProjectNotFoundError(projectID)
	}

	// Open step database
	stepDB, err := projects.OpenProjectStepDatabase(projectID)
	if err != nil {
		return errors.Wrap(errors.ErrDatabaseConnectionFailed, err, "failed to open project step database")
	}
	defer stepDB.Close()

	// Get all steps for the project
	steps, err := stepDB.ListSteps(projectID, false)
	if err != nil {
		return errors.Wrap(errors.ErrDatabaseOperationFailed, err, "failed to list steps")
	}

	if len(steps) == 0 {
		fmt.Printf("No steps found for project '%s'\n", projectID)
		return nil
	}

	// Print header
	fmt.Printf("Steps for project '%s':\n", projectID)
	fmt.Printf("%-8s %-10s %-12s %-10s %-20s %-20s\n", "STEP ID", "STATUS", "DURATION", "EXIT CODE", "STARTED", "COMMIT BEFORE")
	fmt.Printf("%-8s %-10s %-12s %-10s %-20s %-20s\n", "--------", "----------", "------------", "----------", "--------------------", "--------------------")

	// Print each step
	for _, step := range steps {
		status := "COMPLETED"
		if step.Active && step.EndTime == nil {
			status = "RUNNING"
		} else if !step.Active {
			status = "ROLLED BACK"
		}

		duration := "N/A"
		if step.DurationMs != nil && *step.DurationMs > 0 {
			duration = fmt.Sprintf("%dms", *step.DurationMs)
		}

		exitCode := "N/A"
		if step.ExitCode != nil {
			exitCode = fmt.Sprintf("%d", *step.ExitCode)
		}

		started := step.StartTime.Format("2006-01-02 15:04:05")
		commitBefore := step.CommitSHABefore[:8] // Show first 8 characters of SHA

		fmt.Printf("%-8s %-10s %-12s %-10s %-20s %-20s\n",
			fmt.Sprintf("S%d", step.ID), status, duration, exitCode, started, commitBefore)
	}

	return nil
}

// runStepInfo is the handler for the step info command
func runStepInfo(cmd *cobra.Command, args []string) error {
	projectID := args[0]
	stepIDStr := args[1]

	// Validate project ID
	if projectID == "" {
		return errors.NewInvalidInputError("project ID cannot be empty")
	}

	// Parse step ID
	var stepID int
	if _, err := fmt.Sscanf(stepIDStr, "S%d", &stepID); err != nil {
		// Try parsing as plain integer
		if _, err := fmt.Sscanf(stepIDStr, "%d", &stepID); err != nil {
			return errors.NewInvalidInputError(fmt.Sprintf("invalid step ID format: %s. Use format like 'S1' or '1'", stepIDStr))
		}
	}

	// Check if project exists
	exists, err := projects.ProjectExists(projectID)
	if err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to check if project exists")
	}
	if !exists {
		return errors.NewProjectNotFoundError(projectID)
	}

	// Open step database
	stepDB, err := projects.OpenProjectStepDatabase(projectID)
	if err != nil {
		return errors.Wrap(errors.ErrDatabaseConnectionFailed, err, "failed to open project step database")
	}
	defer stepDB.Close()

	// Get step by ID
	step, err := stepDB.GetStep(stepID)
	if err != nil {
		return errors.Wrap(errors.ErrDatabaseOperationFailed, err, "failed to get step")
	}

	if step == nil {
		return errors.NewInvalidInputError(fmt.Sprintf("step S%d not found in project '%s'", stepID, projectID))
	}

	// Print detailed step information
	fmt.Printf("Step Information for S%d (Project: %s)\n", step.ID, projectID)
	fmt.Printf("%s\n", strings.Repeat("=", 55))

	// Basic information
	status := "COMPLETED"
	if step.Active && step.EndTime == nil {
		status = "RUNNING"
	} else if !step.Active {
		status = "ROLLED BACK"
	}
	fmt.Printf("Status: %s\n", status)

	// Timing information
	fmt.Printf("Started: %s\n", step.StartTime.Format("2006-01-02 15:04:05"))
	if step.EndTime != nil {
		fmt.Printf("Ended: %s\n", step.EndTime.Format("2006-01-02 15:04:05"))
	}
	if step.DurationMs != nil && *step.DurationMs > 0 {
		fmt.Printf("Duration: %dms (%.2fs)\n", *step.DurationMs, float64(*step.DurationMs)/1000.0)
	}

	// Exit information
	if step.ExitCode != nil {
		fmt.Printf("Exit Code: %d\n", *step.ExitCode)
	}

	// Commit information
	fmt.Printf("Commit SHA (Before): %s\n", step.CommitSHABefore)
	if step.CommitSHAAfter != "" {
		fmt.Printf("Commit SHA (After): %s\n", step.CommitSHAAfter)
	}

	// Parent step
	if step.ParentStepID != nil {
		fmt.Printf("Parent Step: S%d\n", *step.ParentStepID)
	}

	// Agent configuration
	fmt.Printf("\nAgent Configuration:\n")
	fmt.Printf("  Model: %s\n", step.AgentConfig.Model)
	if step.AgentConfig.MaxTokens > 0 {
		fmt.Printf("  Max Tokens: %d\n", step.AgentConfig.MaxTokens)
	}
	if step.AgentConfig.Temperature > 0 {
		fmt.Printf("  Temperature: %.2f\n", step.AgentConfig.Temperature)
	}
	if len(step.AgentConfig.Metadata) > 0 {
		fmt.Printf("  Metadata:\n")
		for key, value := range step.AgentConfig.Metadata {
			fmt.Printf("    %s: %s\n", key, value)
		}
	}

	// Token usage
	if step.TokenUsage.TotalTokens > 0 {
		fmt.Printf("\nToken Usage:\n")
		fmt.Printf("  Prompt Tokens: %d\n", step.TokenUsage.PromptTokens)
		fmt.Printf("  Completion Tokens: %d\n", step.TokenUsage.CompletionTokens)
		fmt.Printf("  Total Tokens: %d\n", step.TokenUsage.TotalTokens)
		if step.TokenUsage.Cost > 0 {
			fmt.Printf("  Estimated Cost: $%.4f\n", step.TokenUsage.Cost)
		}
	}

	// Creation information
	fmt.Printf("\nCreated: %s\n", step.CreatedAt.Format("2006-01-02 15:04:05"))

	return nil
}

// runStepRollback is the handler for the step rollback command
func runStepRollback(cmd *cobra.Command, args []string) error {
	projectID := args[0]
	stepIDStr := args[1]

	// Validate project ID
	if projectID == "" {
		return errors.NewInvalidInputError("project ID cannot be empty")
	}

	// Parse step ID
	var stepID int
	if _, err := fmt.Sscanf(stepIDStr, "S%d", &stepID); err != nil {
		// Try parsing as plain integer
		if _, err := fmt.Sscanf(stepIDStr, "%d", &stepID); err != nil {
			return errors.NewInvalidInputError(fmt.Sprintf("invalid step ID format: %s. Use format like 'S1' or '1'", stepIDStr))
		}
	}

	// Check if project exists
	exists, err := projects.ProjectExists(projectID)
	if err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to check if project exists")
	}
	if !exists {
		return errors.NewProjectNotFoundError(projectID)
	}

	// Open step database
	stepDB, err := projects.OpenProjectStepDatabase(projectID)
	if err != nil {
		return errors.Wrap(errors.ErrDatabaseConnectionFailed, err, "failed to open project step database")
	}
	defer stepDB.Close()

	// Get step by ID
	targetStep, err := stepDB.GetStep(stepID)
	if err != nil {
		return errors.Wrap(errors.ErrDatabaseOperationFailed, err, "failed to get step")
	}

	if targetStep == nil {
		return errors.NewInvalidInputError(fmt.Sprintf("step S%d not found in project '%s'", stepID, projectID))
	}

	// Validate target step is active
	if !targetStep.Active {
		return errors.NewInvalidInputError(fmt.Sprintf("step S%d is already rolled back", stepID))
	}

	// Get current working directory (should be the git repository)
	repoDir, err := os.Getwd()
	if err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to get current working directory")
	}

	// Verify this is a git repository
	if !git.IsGitRepository(repoDir) {
		return errors.NewInvalidInputError("current directory is not a git repository")
	}

	// Confirm rollback with user
	fmt.Printf("WARNING: This will rollback to step S%d and deactivate all subsequent steps.\n", stepID)
	fmt.Printf("Project: %s\n", projectID)
	fmt.Printf("Target commit: %s\n", targetStep.CommitSHABefore)
	fmt.Print("Proceed with rollback? (yes/no): ")

	var response string
	fmt.Scanln(&response)
	if response != "yes" {
		fmt.Println("Rollback cancelled.")
		return nil
	}

	fmt.Printf("Rolling back to step S%d...\n", stepID)

	// Step 1: Deactivate all steps from the target step ID onwards
	fmt.Printf("Deactivating steps from S%d onwards...\n", stepID)
	if err := stepDB.DeactivateStepsFromID(stepID); err != nil {
		return errors.Wrap(errors.ErrDatabaseOperationFailed, err, "failed to deactivate steps")
	}

	// Step 2: Reset git repository to the commit before the target step
	fmt.Printf("Resetting repository to commit %s...\n", targetStep.CommitSHABefore)
	if err := git.ResetToCommit(repoDir, targetStep.CommitSHABefore); err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to reset repository to target commit")
	}

	fmt.Printf("Successfully rolled back to step S%d\n", stepID)
	fmt.Printf("Repository reset to commit: %s\n", targetStep.CommitSHABefore)
	fmt.Printf("All steps from S%d onwards have been deactivated.\n", stepID)

	return nil
}
