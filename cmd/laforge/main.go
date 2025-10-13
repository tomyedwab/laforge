package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
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
		return fmt.Errorf("failed to create project: %w", err)
	}

	// Get project directory for display
	projectDir, err := projects.GetProjectDir(projectID)
	if err != nil {
		return fmt.Errorf("failed to get project directory: %w", err)
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
	// TODO: Implement step command logic
	return fmt.Errorf("step command not yet implemented")
}
