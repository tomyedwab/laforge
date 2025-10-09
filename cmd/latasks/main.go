package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tomyedwab/laforge/tasks"
)

var rootCmd = &cobra.Command{
	Use:   "latasks",
	Short: "Task management CLI tool",
	Long:  `latasks is a command-line tool for managing tasks with hierarchical structure and work queue functionality.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(nextCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(queueCmd)
	rootCmd.AddCommand(viewCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(reviewCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deleteCmd)
}

func printTask(task *tasks.Task, db *sql.DB) {
	if task == nil {
		return
	}

	fmt.Printf("Task T%d: %s\n", task.ID, task.Title)
	fmt.Printf("Status: %s\n", task.Status)
	if task.ParentID != nil {
		fmt.Printf("Parent: T%d\n", *task.ParentID)
	}
	fmt.Printf("Created: %s\n", task.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated: %s\n", task.UpdatedAt.Format("2006-01-02 15:04:05"))

	// Get and print child tasks
	children, err := tasks.GetChildTasks(db, task.ID)
	if err == nil && len(children) > 0 {
		fmt.Println("\nChildren:")
		for _, child := range children {
			fmt.Printf("  T%d: %s [%s]\n", child.ID, child.Title, child.Status)
		}
	}

	// Get and print task logs
	logs, err := tasks.GetTaskLogs(db, task.ID)
	if err == nil && len(logs) > 0 {
		fmt.Println("\nLogs:")
		for _, log := range logs {
			fmt.Printf("  [%s] %s\n", log.CreatedAt.Format("2006-01-02 15:04:05"), log.Message)
		}
	}

	// Get and print task reviews
	reviews, err := tasks.GetTaskReviews(db, task.ID)
	if err == nil && len(reviews) > 0 {
		fmt.Println("\nReviews:")
		for _, review := range reviews {
			fmt.Printf("  [%s] Status: %s", review.CreatedAt.Format("2006-01-02 15:04:05"), review.Status)
			if review.Attachment != nil {
				fmt.Printf(", Attachment: %s", *review.Attachment)
			}
			fmt.Printf("\n    Message: %s", review.Message)
			if review.Feedback != nil {
				fmt.Printf("\n    Feedback: %s", *review.Feedback)
			}
			fmt.Println()
		}
	}
}

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Retrieve the next task from the upcoming work queue",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := tasks.InitDB()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		task, err := tasks.GetNextTask(db)
		if err != nil {
			return fmt.Errorf("failed to get next task: %w", err)
		}

		if task == nil {
			fmt.Println("No tasks in queue")
			return nil
		}

		printTask(task, db)
		return nil
	},
}

var addCmd = &cobra.Command{
	Use:   "add <title> [parent_id]",
	Short: "Create a new task",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := tasks.InitDB()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		title := args[0]
		var parentID *int

		if len(args) > 1 {
			var pid int
			if _, err := fmt.Sscanf(args[1], "T%d", &pid); err != nil {
				return fmt.Errorf("invalid parent_id format: %s", args[1])
			}
			parentID = &pid
		}

		taskID, err := tasks.AddTask(db, title, parentID)
		if err != nil {
			return fmt.Errorf("failed to add task: %w", err)
		}

		fmt.Printf("Created task T%d\n", taskID)
		return nil
	},
}

var queueCmd = &cobra.Command{
	Use:   "queue <task_id>",
	Short: "Add the task to the upcoming work queue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := tasks.InitDB()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		var taskID int
		if _, err := fmt.Sscanf(args[0], "T%d", &taskID); err != nil {
			return fmt.Errorf("invalid task_id format: %s", args[0])
		}

		if err := tasks.AddToQueue(db, taskID); err != nil {
			return fmt.Errorf("failed to add task to queue: %w", err)
		}

		fmt.Printf("Added T%d to queue\n", taskID)
		return nil
	},
}

var viewCmd = &cobra.Command{
	Use:   "view <task_id>",
	Short: "View details of a specific task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := tasks.InitDB()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		var taskID int
		if _, err := fmt.Sscanf(args[0], "T%d", &taskID); err != nil {
			return fmt.Errorf("invalid task_id format: %s", args[0])
		}

		task, err := tasks.GetTask(db, taskID)
		if err != nil {
			return fmt.Errorf("failed to get task: %w", err)
		}

		if task == nil {
			return fmt.Errorf("task T%d not found", taskID)
		}

		printTask(task, db)
		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update <task_id> <status>",
	Short: "Update the status of a task",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := tasks.InitDB()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		var taskID int
		if _, err := fmt.Sscanf(args[0], "T%d", &taskID); err != nil {
			return fmt.Errorf("invalid task_id format: %s", args[0])
		}

		status := args[1]
		if err := tasks.UpdateTaskStatus(db, taskID, status); err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}

		fmt.Printf("Updated T%d status to %s\n", taskID, status)
		return nil
	},
}

var logCmd = &cobra.Command{
	Use:   "log <task_id> <message>",
	Short: "Update the task log with a summary of what was done and what work remains",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := tasks.InitDB()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		var taskID int
		if _, err := fmt.Sscanf(args[0], "T%d", &taskID); err != nil {
			return fmt.Errorf("invalid task_id format: %s", args[0])
		}

		message := args[1]
		if err := tasks.AddTaskLog(db, taskID, message); err != nil {
			return fmt.Errorf("failed to add task log: %w", err)
		}

		fmt.Printf("Added log to T%d\n", taskID)
		return nil
	},
}

var reviewCmd = &cobra.Command{
	Use:   "review <task_id> <message> [attachment]",
	Short: "Send a review request and move the task to in-review",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := tasks.InitDB()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		var taskID int
		if _, err := fmt.Sscanf(args[0], "T%d", &taskID); err != nil {
			return fmt.Errorf("invalid task_id format: %s", args[0])
		}

		message := args[1]
		var attachment *string
		if len(args) > 2 {
			attachment = &args[2]
		}

		if err := tasks.CreateReview(db, taskID, message, attachment); err != nil {
			return fmt.Errorf("failed to create review: %w", err)
		}

		fmt.Printf("Created review for T%d\n", taskID)
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := tasks.InitDB()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		tasks, err := tasks.ListTasks(db)
		if err != nil {
			return fmt.Errorf("failed to list tasks: %w", err)
		}

		if len(tasks) == 0 {
			fmt.Println("No tasks found")
			return nil
		}

		for _, task := range tasks {
			fmt.Printf("T%d: %s [%s]", task.ID, task.Title, task.Status)
			if task.ParentID != nil {
				fmt.Printf(" (parent: T%d)", *task.ParentID)
			}
			fmt.Println()
		}

		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <task_id>",
	Short: "Delete a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := tasks.InitDB()
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer db.Close()

		var taskID int
		if _, err := fmt.Sscanf(args[0], "T%d", &taskID); err != nil {
			return fmt.Errorf("invalid task_id format: %s", args[0])
		}

		if err := tasks.DeleteTask(db, taskID); err != nil {
			return fmt.Errorf("failed to delete task: %w", err)
		}

		fmt.Printf("Deleted T%d\n", taskID)
		return nil
	},
}
