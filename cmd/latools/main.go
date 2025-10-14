package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
	rootCmd.AddCommand(reviewCmd)
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

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Interactively review pending task reviews",
	Long: `Review pending task reviews by paging through each one and providing approval/rejection status.

For each pending review, you will see:
- The task information (ID and title)
- The review message
- Any attached file path
- A prompt to approve, reject, or skip

Examples:
  latools review
  latools review --db /path/to/tasks.db
  TASKS_DB_PATH=/path/to/tasks.db latools review`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Initialize database
		db, err := tasks.InitDB()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		// Get pending reviews
		reviews, err := tasks.GetPendingReviews(db)
		if err != nil {
			return fmt.Errorf("failed to get pending reviews: %w", err)
		}

		if len(reviews) == 0 {
			fmt.Println("No pending reviews found!")
			return nil
		}

		fmt.Printf("Found %d pending review(s)\n\n", len(reviews))

		scanner := bufio.NewScanner(os.Stdin)
		reviewedCount := 0

		for i, review := range reviews {
			// Get the task information
			task, err := tasks.GetTask(db, review.TaskID)
			if err != nil {
				fmt.Printf("Error getting task T%d: %v\n", review.TaskID, err)
				continue
			}
			if task == nil {
				fmt.Printf("Task T%d not found\n", review.TaskID)
				continue
			}

			// Display review information
			fmt.Printf("═══════════════════════════════════════════════════════════════════\n")
			fmt.Printf("Review %d of %d (ID: %d)\n", i+1, len(reviews), review.ID)
			fmt.Printf("═══════════════════════════════════════════════════════════════════\n")
			fmt.Printf("Task:        T%d - %s\n", task.ID, task.Title)
			fmt.Printf("Task Status: %s\n", task.Status)
			fmt.Printf("Created:     %s\n\n", review.CreatedAt.Format("2006-01-02 15:04:05"))

			fmt.Printf("Message:\n%s\n\n", review.Message)

			if review.Attachment != nil && *review.Attachment != "" {
				fmt.Printf("Attachment:  %s\n", *review.Attachment)

				// Try to display the file if it exists
				attachmentPath := *review.Attachment
				_, err := os.Stat(attachmentPath)

				// If not found and path starts with "/src", try without that prefix
				if err != nil && strings.HasPrefix(attachmentPath, "/src/") {
					alternativePath := strings.TrimPrefix(attachmentPath, "/src/")
					if _, altErr := os.Stat(alternativePath); altErr == nil {
						attachmentPath = alternativePath
						err = nil
						fmt.Printf("             (using %s)\n", alternativePath)
					}
				}

				if err == nil {
					fmt.Println("\nFile contents:")
					fmt.Println("───────────────────────────────────────────────────────────────────")
					if content, err := os.ReadFile(attachmentPath); err == nil {
						// Limit display to first 2000 characters
						contentStr := string(content)
						if len(contentStr) > 2000 {
							fmt.Printf("%s\n... (truncated, file is %d bytes)\n", contentStr[:2000], len(content))
						} else {
							fmt.Println(contentStr)
						}
					} else {
						fmt.Printf("(Could not read file: %v)\n", err)
					}
					fmt.Println("───────────────────────────────────────────────────────────────────")
				} else {
					fmt.Printf("(File not found at path: %s)\n", *review.Attachment)
				}
				fmt.Println()
			}

			// Prompt for action
			for {
				fmt.Print("Action [a]pprove, [r]eject, [s]kip, [q]uit: ")
				if !scanner.Scan() {
					return fmt.Errorf("failed to read input")
				}
				action := strings.ToLower(strings.TrimSpace(scanner.Text()))

				if action == "q" || action == "quit" {
					fmt.Printf("\nReviewed %d of %d reviews.\n", reviewedCount, len(reviews))
					return nil
				}

				if action == "s" || action == "skip" {
					fmt.Println("Skipped.\n")
					break
				}

				if action != "a" && action != "approve" && action != "r" && action != "reject" {
					fmt.Println("Invalid action. Please enter 'a' (approve), 'r' (reject), 's' (skip), or 'q' (quit).")
					continue
				}

				// Determine status
				status := "approved"
				if action == "r" || action == "reject" {
					status = "rejected"
				}

				// Prompt for feedback
				fmt.Print("Feedback (optional, press Enter to skip): ")
				if !scanner.Scan() {
					return fmt.Errorf("failed to read input")
				}
				feedbackText := strings.TrimSpace(scanner.Text())
				var feedback *string
				if feedbackText != "" {
					feedback = &feedbackText
				}

				// Update the review
				if err := tasks.UpdateReview(db, review.ID, status, feedback); err != nil {
					return fmt.Errorf("failed to update review: %w", err)
				}

				fmt.Printf("Review %s!\n\n", status)
				reviewedCount++
				break
			}
		}

		fmt.Printf("Review session complete. Reviewed %d of %d reviews.\n", reviewedCount, len(reviews))
		return nil
	},
}
