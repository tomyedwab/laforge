package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tomyedwab/laforge/database"
	"github.com/tomyedwab/laforge/docker"
	"github.com/tomyedwab/laforge/errors"
	"github.com/tomyedwab/laforge/git"
	"github.com/tomyedwab/laforge/logging"
	"github.com/tomyedwab/laforge/projects"
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
}

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init [project-id]",
	Short: "Initialize a new LaForge project",
	Long: `Initialize a new LaForge project with the specified ID.
	
This command creates the project directory structure, initializes a git repository,
creates a task database, and generates the project configuration file.`,
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

func init() {
	// Add flags for init command
	initCmd.Flags().String("name", "", "project name")
	initCmd.Flags().String("description", "", "project description")

	// Add flags for step command
	stepCmd.Flags().String("agent-image", "laforge-agent:latest", "Docker image for the agent container")
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

	// Use project ID as name if name is not provided
	if name == "" {
		name = projectID
	}

	// Create the project
	project, err := projects.CreateProject(projectID, name, description)
	if err != nil {
		return errors.Wrap(errors.ErrProjectAlreadyExists, err, "failed to create project")
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
	agentImage, _ := cmd.Flags().GetString("agent-image")
	timeout, _ := cmd.Flags().GetDuration("timeout")
	verbose, _ := cmd.Flags().GetBool("verbose")
	quiet, _ := cmd.Flags().GetBool("quiet")

	// Validate agent image
	if agentImage == "" {
		agentImage = "laforge-agent:latest"
	}

	// Check if project exists
	exists, err := projects.ProjectExists(projectID)
	if err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to check if project exists")
	}
	if !exists {
		return errors.NewProjectNotFoundError(projectID)
	}

	// Get project directory
	projectDir, err := projects.GetProjectDir(projectID)
	if err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to get project directory")
	}

	// Get task database path
	taskDBPath, err := projects.GetProjectTaskDatabase(projectID)
	if err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to get project task database path")
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

	// Generate step ID
	stepID := logging.GenerateStepID()
	stepLogger := logging.NewStepLogger(logger, projectID, stepID)

	// Log step start
	stepLogger.LogStepStart(projectID)
	stepStartTime := time.Now()

	defer func() {
		// Log step completion
		duration := time.Since(stepStartTime)
		stepLogger.LogStepEnd(err == nil, duration, 0)
	}()

	// Step 1: Create temporary git worktree
	stepLogger.LogStepPhase("worktree", "Creating temporary git worktree")
	worktree, err := git.CreateTempWorktree(projectDir, "step")
	if err != nil {
		stepLogger.LogError("git", "Failed to create temporary worktree", err, map[string]interface{}{
			"project_dir": projectDir,
		})
		return errors.Wrap(errors.ErrUnknown, err, "failed to create temporary worktree")
	}
	stepLogger.LogWorktreeCreation(worktree.Path, "step")
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

	// Step 4: Launch agent container
	stepLogger.LogStepPhase("container", "Launching agent container")
	containerName := fmt.Sprintf("laforge-agent-%s-%d", projectID, time.Now().Unix())
	containerConfig := &docker.ContainerConfig{
		Image:      agentImage,
		Name:       containerName,
		WorkDir:    worktree.Path,
		TaskDBPath: tempDBPath,
		Environment: map[string]string{
			"LAFORGE_PROJECT_ID": projectID,
			"LAFORGE_STEP":       "true",
		},
		AutoRemove: true,
		Timeout:    timeout,
	}

	stepLogger.LogContainerLaunch(agentImage, containerName, map[string]interface{}{
		"work_dir":     worktree.Path,
		"task_db_path": tempDBPath,
		"timeout":      timeout.String(),
	})

	// Run the agent container and wait for completion
	containerMetrics := &docker.ContainerMetrics{}
	exitCode, logs, err := dockerClient.RunAgentContainerWithMetrics(containerConfig, containerMetrics)
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

	// Step 5: Check if there are changes to commit
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
		commitMessage := fmt.Sprintf("LaForge step %s - Automated changes", stepID)
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

	// Step 6: Copy changes back from temporary database to main database
	stepLogger.LogStepPhase("database", "Updating main task database")
	stepLogger.LogDatabaseUpdate(tempDBPath, taskDBPath)
	if err := database.CopyDatabase(tempDBPath, taskDBPath); err != nil {
		stepLogger.LogError("database", "Failed to update main task database", err, map[string]interface{}{
			"source_path": tempDBPath,
			"dest_path":   taskDBPath,
		})
		return errors.Wrap(errors.ErrUnknown, err, "failed to update main task database")
	}

	logger.Info("step", fmt.Sprintf("LaForge step completed successfully for project '%s'", projectID), map[string]interface{}{
		"project_id": projectID,
		"step_id":    stepID,
		"duration":   time.Since(stepStartTime).String(),
	})

	return nil
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

// commitChanges commits all changes in the repository with the given message
func commitChanges(repoDir string, message string) error {
	// Add all changes
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add changes: %w", err)
	}

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
