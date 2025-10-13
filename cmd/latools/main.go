package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tomyedwab/laforge/tasks"
)

var rootCmd = &cobra.Command{
	Use:   "latools",
	Short: "Task management utilities",
	Long:  `latools provides utilities for managing tasks, including bulk import from YAML files.`,
}

var dbPath string

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", "", "Path to SQLite database file (defaults to TASKS_DB_PATH env var or /state/tasks.db)")

	// Add commands
	rootCmd.AddCommand(importCmd)
}

var importCmd = &cobra.Command{
	Use:   "import <yaml_file>",
	Short: "Import tasks from a YAML file into the database",
	Long: `Import tasks from a YAML file into the SQLite database.

The YAML file should contain tasks in the format specified in docs/examples/tasks.yml.
Tasks will be imported with auto-generated IDs, and references (parent_id, upstream_dependency_id)
will be automatically resolved.

Examples:
  latools import tasks.yml
  latools import --db /path/to/tasks.db tasks.yml
  TASKS_DB_PATH=/path/to/tasks.db latools import tasks.yml`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		yamlFile := args[0]

		// Check if YAML file exists
		if _, err := os.Stat(yamlFile); os.IsNotExist(err) {
			return fmt.Errorf("YAML file not found: %s", yamlFile)
		}

		// Determine database path
		finalDBPath := dbPath
		if finalDBPath == "" {
			finalDBPath = os.Getenv("TASKS_DB_PATH")
			if finalDBPath == "" {
				finalDBPath = "/state/tasks.db"
			}
		}

		// Set the environment variable so InitDB uses the correct path
		originalDBPath := os.Getenv("TASKS_DB_PATH")
		os.Setenv("TASKS_DB_PATH", finalDBPath)
		defer func() {
			if originalDBPath != "" {
				os.Setenv("TASKS_DB_PATH", originalDBPath)
			} else {
				os.Unsetenv("TASKS_DB_PATH")
			}
		}()

		// Initialize database to ensure schema exists
		db, err := tasks.InitDB()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		// Import tasks from YAML
		fmt.Printf("Importing tasks from %s into %s...\n", yamlFile, finalDBPath)
		if err := tasks.ImportTasksFromYAML(db, yamlFile); err != nil {
			return fmt.Errorf("failed to import tasks: %w", err)
		}

		fmt.Println("Successfully imported tasks!")

		// Show summary
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&count); err == nil {
			fmt.Printf("Total tasks in database: %d\n", count)
		}

		return nil
	},
}
