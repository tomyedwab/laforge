package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tomyedwab/laforge/lib/tasks"
)

var rootCmd = &cobra.Command{
	Use:   "latasks",
	Short: "Task management CLI tool",
	Long:  `latasks is a command-line tool for managing tasks with hierarchical structure and dependency tracking.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(nextCmd)
	rootCmd.AddCommand(viewCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(leaseCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(reviewCmd)
}

func sendRequest(endpoint, method string, body interface{}, response interface{}) error {
	urlPath, found := os.LookupEnv("LATASK_URLPATH")
	if !found || urlPath == "" {
		return fmt.Errorf("LATASK_URLPATH environment variable not set")
	}

	token, found := os.LookupEnv("LATASK_TOKEN")
	if !found || token == "" {
		return fmt.Errorf("LATASK_TOKEN environment variable not set")
	}

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
	req, err := http.NewRequest(method, fmt.Sprintf("%s%s", urlPath, endpoint), reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
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

func printTask(task *tasks.TaskResponse, children []*tasks.TaskResponse, logs []*tasks.TaskLogResponse, reviews []*tasks.TaskReviewResponse) {
	fmt.Printf("Task T%d: %s\n", task.ID, task.Title)
	fmt.Printf("Status: %s\n", task.Status)
	if task.Description != "" {
		fmt.Printf("Description: %s\n", task.Description)
	}
	if task.AcceptanceCriteria != "" {
		fmt.Printf("Acceptance Criteria:\n%s\n", task.AcceptanceCriteria)
	}
	if task.UpstreamDependencyID != nil {
		fmt.Printf("Upstream Dependency: T%d\n", *task.UpstreamDependencyID)
	}
	if task.ReviewRequired {
		fmt.Printf("Review Required: Yes\n")
	}
	if task.ParentID != nil {
		fmt.Printf("Parent: T%d\n", *task.ParentID)
	}
	fmt.Printf("Created: %s\n", task.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated: %s\n", task.UpdatedAt.Format("2006-01-02 15:04:05"))

	// Print child tasks
	if len(children) > 0 {
		fmt.Println("\nChildren:")
		for _, child := range children {
			fmt.Printf("  T%d: %s [%s]\n", child.ID, child.Title, child.Status)
		}
	}

	// Print task logs
	if len(logs) > 0 {
		fmt.Println("\nLogs:")
		for _, log := range logs {
			fmt.Printf("  [%s] %s\n", log.CreatedAt.Format("2006-01-02 15:04:05"), log.Message)
		}
	}

	// Print task reviews
	if len(reviews) > 0 {
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

	fmt.Println()
}

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Retrieve the next task that is ready for work",
	Long:  "Returns tasks in 'todo', 'in-progress', or 'in-review' status (with no pending reviews) where all upstream dependencies are completed",
	RunE: func(cmd *cobra.Command, args []string) error {
		var singleTaskResponse tasks.SingleTaskResponse
		err := sendRequest("/tasks/next?include_children=true&include_logs=true&include_reviews=true", "GET", nil, &singleTaskResponse)
		if err != nil {
			return fmt.Errorf("failed to fetch task: %w", err)
		}

		printTask(singleTaskResponse.Task, singleTaskResponse.TaskChildren, singleTaskResponse.TaskLogs, singleTaskResponse.TaskReviews)
		return nil
	},
}

var viewCmd = &cobra.Command{
	Use:   "view <task_id>",
	Short: "View details of a specific task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var taskID int
		if _, err := fmt.Sscanf(args[0], "T%d", &taskID); err != nil {
			return fmt.Errorf("invalid task_id format: %s", args[0])
		}

		var singleTaskResponse tasks.SingleTaskResponse
		err := sendRequest(fmt.Sprintf("/tasks/%d?include_children=true&include_logs=true&include_reviews=true", taskID), "GET", nil, &singleTaskResponse)
		if err != nil {
			return fmt.Errorf("failed to fetch task: %w", err)
		}

		printTask(singleTaskResponse.Task, singleTaskResponse.TaskChildren, singleTaskResponse.TaskLogs, singleTaskResponse.TaskReviews)
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list [page] [limit]",
	Short: "List all tasks",
	Args:  cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		page := 1
		limit := 25

		if len(args) > 0 {
			if _, err := fmt.Sscanf(args[0], "%d", &page); err != nil {
				return fmt.Errorf("invalid page format: %s", args[0])
			}
		}
		if len(args) > 1 {
			if _, err := fmt.Sscanf(args[1], "%d", &limit); err != nil {
				return fmt.Errorf("invalid limit format: %s", args[1])
			}
		}

		var taskListResponse tasks.TaskListResponse
		err := sendRequest(fmt.Sprintf("/tasks?page=%d&limit=%d", page, limit), "GET", nil, &taskListResponse)
		if err != nil {
			return fmt.Errorf("failed to fetch tasks: %w", err)
		}

		tasks := taskListResponse.Data.Tasks
		if len(tasks) == 0 {
			fmt.Println("No tasks found")
			return nil
		}

		for _, task := range tasks {
			fmt.Printf("T%d: %s [%s]", task.ID, task.Title, task.Status)
			if task.ParentID != nil {
				fmt.Printf(" (parent: T%d)", *task.ParentID)
			}
			if task.UpstreamDependencyID != nil {
				fmt.Printf(" (depends on: T%d)", *task.UpstreamDependencyID)
			}
			if task.ReviewRequired {
				fmt.Printf(" [review required]")
			}
			fmt.Println()

			if task.Description != "" {
				fmt.Printf("  Description: %s\n", task.Description)
			}
			if task.AcceptanceCriteria != "" {
				fmt.Printf("  Acceptance Criteria: %s\n", task.AcceptanceCriteria)
			}
		}

		fmt.Printf("Showing page %d of %d\n", page, taskListResponse.Data.Pagination.Pages)

		return nil
	},
}

var leaseCmd = &cobra.Command{
	Use:   "lease <task_id>",
	Short: "Lease a task for use in this step",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var taskID int
		if _, err := fmt.Sscanf(args[0], "T%d", &taskID); err != nil {
			return fmt.Errorf("invalid task_id format: %s", args[0])
		}

		var statusResponse struct {
			Status string `json:"status"`
		}
		err := sendRequest(fmt.Sprintf("/tasks/%d/lease", taskID), "POST", nil, &statusResponse)
		if err != nil {
			return fmt.Errorf("failed to fetch task: %w", err)
		}

		fmt.Printf("Leased task %d\n", taskID)
		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update <task_id> <status>",
	Short: "Update the status of a task",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var taskID int
		if _, err := fmt.Sscanf(args[0], "T%d", &taskID); err != nil {
			return fmt.Errorf("invalid task_id format: %s", args[0])
		}
		status := args[1]

		var statusResponse struct {
			Status string `json:"status"`
		}
		err := sendRequest(fmt.Sprintf("/tasks/%d/queue", taskID), "POST", &tasks.TaskQueuedUpdateRequest{
			Status: &status,
		}, &statusResponse)
		if err != nil {
			return fmt.Errorf("failed to queue task update: %w", err)
		}

		fmt.Printf("Queued update of task %d status to %s\n", taskID, status)
		return nil
	},
}

var logCmd = &cobra.Command{
	Use:   "log <task_id> <message>",
	Short: "Update the task log with a summary of what was done and what work remains",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var taskID int
		if _, err := fmt.Sscanf(args[0], "T%d", &taskID); err != nil {
			return fmt.Errorf("invalid task_id format: %s", args[0])
		}
		message := args[1]

		var statusResponse struct {
			Status string `json:"status"`
		}
		err := sendRequest(fmt.Sprintf("/tasks/%d/queue", taskID), "POST", &tasks.TaskQueuedUpdateRequest{
			LogMessage: &tasks.TaskQueuedLogRequest{
				Message:   message,
				CreatedAt: time.Now(),
			},
		}, &statusResponse)
		if err != nil {
			return fmt.Errorf("failed to queue task update: %w", err)
		}

		fmt.Printf("Queued write of log to task %d\n", taskID)
		return nil
	},
}

var reviewCmd = &cobra.Command{
	Use:   "review <task_id> <message> [attachment]",
	Short: "Send a review request and move the task to in-review",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		var taskID int
		if _, err := fmt.Sscanf(args[0], "T%d", &taskID); err != nil {
			return fmt.Errorf("invalid task_id format: %s", args[0])
		}

		message := args[1]
		var attachment *string
		if len(args) > 2 {
			attachment = &args[2]
		}

		var statusResponse struct {
			Status string `json:"status"`
		}
		err := sendRequest(fmt.Sprintf("/tasks/%d/queue", taskID), "POST", &tasks.TaskQueuedUpdateRequest{
			Review: &tasks.TaskQueuedReviewRequest{
				Message:    message,
				CreatedAt:  time.Now(),
				Attachment: attachment,
			},
		}, &statusResponse)
		if err != nil {
			return fmt.Errorf("failed to queue task update: %w", err)
		}

		fmt.Printf("Submitted review for task %d\n", taskID)
		return nil
	},
}
