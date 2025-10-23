package main

import (
	"bytes"
	"encoding/json"
	nativeerrors "errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tomyedwab/laforge/lib/docker"
	"github.com/tomyedwab/laforge/lib/errors"
	"github.com/tomyedwab/laforge/lib/git"
	"github.com/tomyedwab/laforge/lib/logging"
	"github.com/tomyedwab/laforge/lib/projects"
	"github.com/tomyedwab/laforge/lib/steps"
)

var (
	version  = "dev"
	commit   = "none"
	date     = "unknown"
	apiToken = ""
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

func init() {
	// Add flags for init command
	initCmd.Flags().String("name", "", "project name")
	initCmd.Flags().String("description", "", "project description")
	initCmd.Flags().String("agent-image", "", "default Docker image for the agent container")
	initCmd.Flags().String("agent-config-file", "", "path to custom agents.yml configuration file")
	initCmd.Flags().String("main-branch", "main", "main branch name for automerging step commits")

	// Add flags for step command
	stepCmd.Flags().String("agent-config", "", "agent configuration name from agents.yml (overrides --agent-image)")
	stepCmd.Flags().Duration("timeout", 0, "timeout for step execution (0 means no timeout)")
}

var (
	MissingCredentialsError = nativeerrors.New("missing credentials")
	InvalidCredentialsError = nativeerrors.New("invalid credentials")
)

func sendRequestAttempt(url, method string, body interface{}, response interface{}) error {
	client := &http.Client{}
	var reqBody io.Reader
	if body != nil {
		// Serialize the body to JSON
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return err
	}
	if apiToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiToken))
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return InvalidCredentialsError
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		// Read body text
		bodyText, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("unexpected status code: %d. Message: %s", resp.StatusCode, string(bodyText))
	}
	if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
		return err
	}
	return nil
}

func sendRequest(project, endpoint, method string, body interface{}, response interface{}) error {
	urlPrefix, found := os.LookupEnv("LAFORGE_URLPATH")
	if !found || urlPrefix == "" {
		urlPrefix = "http://localhost:8080/api/v1"
	}
	urlPath := fmt.Sprintf("%s/projects/%s", urlPrefix, project)

	var err error
	for _ = range 3 {
		err = sendRequestAttempt(fmt.Sprintf("%s%s", urlPath, endpoint), method, body, response)
		if err == nil {
			return nil
		}
		if err == InvalidCredentialsError || err == MissingCredentialsError {
			log.Printf("Logging in...")
			var loginResponse struct {
				Token  string `json:"token"`
				UserID string `json:"user_id"`
			}
			err = sendRequestAttempt(fmt.Sprintf("%s/public/login", urlPrefix), "POST", nil, &loginResponse)
			if err != nil {
				return err
			}
			apiToken = loginResponse.Token
			err = nativeerrors.New("Failed after three login attempts")
		} else {
			return err
		}
	}
	return err
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
	mainBranch, _ := cmd.Flags().GetString("main-branch")

	// Use project ID as name if name is not provided
	if name == "" {
		name = projectID
	}

	// Get current working directory as repository path
	repositoryPath, err := os.Getwd()
	if err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to get current working directory")
	}

	// Create the project
	project, err := projects.CreateProject(projectID, name, description, repositoryPath, mainBranch)
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

	// Load project configuration to get main branch
	project, err := projects.LoadProject(projectID)
	if err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to load project configuration")
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

	// Get current commit SHA before step execution
	commitSHABefore, err := git.GetCurrentCommitSHA(sourceDir)
	if err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to get current commit SHA")
	}

	var leaseResponse steps.LeaseStepResponse
	err = sendRequest(projectID, "/steps/lease", "POST", &steps.LeaseStepRequest{
		CommitSHABefore: commitSHABefore,
		AgentConfigName: agentConfig.Name,
	}, &leaseResponse)
	if err != nil {
		return errors.Wrap(errors.ErrUnknown, err, "failed to lease step")
	}

	stepID := leaseResponse.StepID
	dbStepID := fmt.Sprintf("S%d", stepID)
	stepLogger := logging.NewStepLogger(logger, projectID, dbStepID)

	// Log step start
	stepLogger.LogStepStart(projectID)
	stepStartTime := time.Now()

	// Declare worktree variable for use in defer
	var worktree *git.Worktree

	defer func() {
		// Update step record with completion data
		exitCode := int(0)
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

		var successResponse struct {
			Status string `json:"status"`
		}
		err = sendRequest(projectID, "/steps/finalize", "POST", &steps.FinalizeStepRequest{
			StepID:         stepID,
			CommitSHAAfter: commitSHAAfter,
			ExitCode:       exitCode,
		}, &successResponse)
		if err != nil {
			stepLogger.LogError("database", "Failed to update step record", err, map[string]interface{}{
				"step_id": stepID,
			})
		}

		// Log step completion
		stepLogger.LogStepEnd(err == nil, exitCode)
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
	worktreeRemoved := false
	defer func() {
		// Clean up worktree only if it hasn't been removed yet
		if !worktreeRemoved {
			stepLogger.LogWorktreeCleanup(worktree.Path)
			if cleanupErr := git.RemoveWorktree(worktree); cleanupErr != nil {
				stepLogger.LogWarning("git", "Failed to remove worktree", map[string]interface{}{
					"error": cleanupErr.Error(),
				})
			}
		}
	}()

	// Step 2: Create Docker client
	stepLogger.LogStepPhase("docker", "Initializing Docker client")
	dockerClient, err := docker.NewClient()
	if err != nil {
		stepLogger.LogError("docker", "Failed to create Docker client", err, nil)
		return errors.Wrap(errors.ErrUnknown, err, "failed to create Docker client")
	}
	stepLogger.LogDockerClientInit()
	defer dockerClient.Close()

	// Step 3: Create log file for streaming container output
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

	// Wrap with formatting writer to convert JSON output to markdown
	formattedWriter := docker.NewFormattingWriter(multiWriter)

	// Step 4: Launch agent container
	stepLogger.LogStepPhase("container", "Launching agent container")
	containerMetrics := &docker.ContainerMetrics{}

	var exitCode int64
	var logs string

	// Use agent configuration from agents.yml
	stepLogger.LogContainerLaunch(agentConfig.Image, fmt.Sprintf("laforge-agent-%s-%s-%d", projectID, agentConfig.Name, time.Now().Unix()), map[string]interface{}{
		"work_dir":     worktree.Path,
		"agent_config": agentConfigName,
		"timeout":      agentConfig.Runtime.Timeout,
		"log_file":     logFilePath,
	})

	// Run container using AgentConfig with streaming logs (formatted as markdown)
	exitCode, logs, err = dockerClient.RunAgentContainerFromConfigWithStreamingLogs(agentConfig, worktree.Path, projectID, leaseResponse.Token, formattedWriter, containerMetrics)
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
		// Check if there are actual changes besides COMMIT.md
		hasActualChanges, err := hasGitChangesExcludingCommitMD(worktree.Path)
		if err != nil {
			stepLogger.LogWarning("git", "Failed to check for git changes excluding COMMIT.md", map[string]interface{}{
				"error": err.Error(),
			})
			// Continue anyway - try to commit
			hasActualChanges = true
		}

		if hasActualChanges {
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
			if err := commitChanges(worktree.Path, commitMessage, dbStepID); err != nil {
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
			logger.Info("git", "No changes to commit (only COMMIT.md was modified)", map[string]interface{}{
				"project_id": projectID,
				"step_id":    stepID,
			})
		}
	} else {
		logger.Info("git", "No changes to commit", map[string]interface{}{
			"project_id": projectID,
			"step_id":    stepID,
		})
	}

	// Delete the COMMIT.md file if it exists
	commitMDPath := filepath.Join(worktree.Path, "COMMIT.md")
	if err := os.Remove(commitMDPath); err != nil {
		stepLogger.LogWarning("git", "Failed to delete COMMIT.md", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Step 6: Automerge step branch into main branch (only if step completed successfully)
	if exitCode == 0 && hasChanges {
		stepLogger.LogStepPhase("git", fmt.Sprintf("Automerging step branch into %s branch", project.MainBranch))

		stepBranch := fmt.Sprintf("step-S%d", stepID)
		mergeMessage := fmt.Sprintf("Automerge %s into %s", stepBranch, project.MainBranch)

		if mergeErr := git.MergeBranch(sourceDir, stepBranch, project.MainBranch, mergeMessage); mergeErr != nil {
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
			// Merge successful, clean up worktree first, then delete step branch
			stepLogger.LogStepPhase("git", "Merge successful, cleaning up worktree and step branch")

			// Remove worktree before attempting branch deletion
			stepLogger.LogWorktreeCleanup(worktree.Path)
			if cleanupErr := git.RemoveWorktree(worktree); cleanupErr != nil {
				stepLogger.LogWarning("git", "Failed to remove worktree after successful merge", map[string]interface{}{
					"error": cleanupErr.Error(),
				})
			} else {
				worktreeRemoved = true // Mark worktree as removed to prevent double cleanup in defer
			}

			// Now delete the step branch
			if deleteErr := git.DeleteBranch(sourceDir, stepBranch); deleteErr != nil {
				stepLogger.LogWarning("git", "Failed to delete step branch after successful merge", map[string]interface{}{
					"step_branch": stepBranch,
					"error":       deleteErr.Error(),
				})
			} else {
				logger.Info("git", fmt.Sprintf("Successfully automerged %s into %s and deleted step branch", stepBranch, project.MainBranch), map[string]interface{}{
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

// hasGitChangesExcludingCommitMD checks if there are uncommitted changes excluding COMMIT.md
func hasGitChangesExcludingCommitMD(repoDir string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	// Filter out COMMIT.md from the output
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line != "" && !strings.HasSuffix(line, "COMMIT.md") {
			return true, nil
		}
	}

	return false, nil
}

// commitChanges commits all changes in the repository with the given message,
// excluding COMMIT.md if it exists, and adds a git note with the step ID
func commitChanges(repoDir string, message string, stepID string) error {
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

	// Add git note with step ID
	noteMessage := fmt.Sprintf("Step-id: %s", stepID)
	cmd = exec.Command("git", "notes", "add", "-m", noteMessage)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add git note: %w", err)
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
	fmt.Printf("\nAgent Configuration: %s\n", step.AgentConfigName)

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
