package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
	"github.com/tomyedwab/laforge/tasks"
)

func setupTestCLI(t *testing.T) (*cobra.Command, *sql.DB, func()) {
	// Create in-memory database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create schema
	if err := createTestSchema(db); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Create a test command that uses our database
	testCmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	cleanup := func() {
		db.Close()
	}

	return testCmd, db, cleanup
}

func createTestSchema(db *sql.DB) error {
	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		description TEXT DEFAULT '',
		acceptance_criteria TEXT DEFAULT '',
		upstream_dependency_id INTEGER,
		review_required BOOLEAN DEFAULT FALSE,
		parent_id INTEGER,
		status TEXT NOT NULL DEFAULT 'todo',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (parent_id) REFERENCES tasks(id) ON DELETE CASCADE,
		FOREIGN KEY (upstream_dependency_id) REFERENCES tasks(id) ON DELETE SET NULL,
		CHECK (status IN ('todo', 'in-progress', 'in-review', 'completed'))
	);

	CREATE TABLE IF NOT EXISTS task_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id INTEGER NOT NULL,
		message TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS task_reviews (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id INTEGER NOT NULL,
		message TEXT NOT NULL,
		attachment TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		feedback TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
		CHECK (status IN ('pending', 'approved', 'rejected'))
	);`

	_, err := db.Exec(schema)
	return err
}

func TestAddCommand(t *testing.T) {
	_, db, cleanup := setupTestCLI(t)
	defer cleanup()

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		wantOut string
	}{
		{
			name:    "Add simple task",
			args:    []string{"Test Task"},
			wantErr: false,
			wantOut: "Created task T1",
		},
		{
			name:    "Add task with parent",
			args:    []string{"Child Task", "T1"},
			wantErr: false,
			wantOut: "Created task T2",
		},
		{
			name:    "Add task with invalid parent format",
			args:    []string{"Child Task", "invalid"},
			wantErr: true,
			wantOut: "invalid parent_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture output
			var buf bytes.Buffer

			// Create a test add command
			cmd := &cobra.Command{
				Use:  "add",
				Args: cobra.RangeArgs(1, 2),
				RunE: func(cmd *cobra.Command, args []string) error {
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

					fmt.Fprintf(&buf, "Created task T%d\n", taskID)
					return nil
				},
			}

			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !strings.Contains(buf.String(), tt.wantOut) {
				t.Errorf("Output = %v, want output containing %v", buf.String(), tt.wantOut)
			}
		})
	}
}

func TestListCommand(t *testing.T) {
	_, db, cleanup := setupTestCLI(t)
	defer cleanup()

	// Add some test tasks
	taskTitles := []string{"Task 1", "Task 2", "Task 3"}
	for _, title := range taskTitles {
		_, err := tasks.AddTask(db, title, nil)
		if err != nil {
			t.Fatalf("Failed to add test task: %v", err)
		}
	}

	// Create a buffer to capture output
	var buf bytes.Buffer

	// Create a test list command
	cmd := &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			taskList, err := tasks.ListTasks(db)
			if err != nil {
				return fmt.Errorf("failed to list tasks: %w", err)
			}

			if len(taskList) == 0 {
				fmt.Fprintln(&buf, "No tasks found")
				return nil
			}

			for _, task := range taskList {
				fmt.Fprintf(&buf, "T%d: %s [%s]\n", task.ID, task.Title, task.Status)
			}

			return nil
		},
	}

	err := cmd.Execute()
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}

	// Check that all tasks are listed
	output := buf.String()
	for i, title := range taskTitles {
		expected := fmt.Sprintf("T%d: %s [todo]", i+1, title)
		if !strings.Contains(output, expected) {
			t.Errorf("Output should contain %v, got %v", expected, output)
		}
	}
}

func TestViewCommand(t *testing.T) {
	_, db, cleanup := setupTestCLI(t)
	defer cleanup()

	// Add a test task
	_, err := tasks.AddTask(db, "Test Task", nil)
	if err != nil {
		t.Fatalf("Failed to add test task: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		wantOut string
	}{
		{
			name:    "View existing task",
			args:    []string{"T1"},
			wantErr: false,
			wantOut: "Task T1: Test Task",
		},
		{
			name:    "View non-existent task",
			args:    []string{"T999"},
			wantErr: true,
			wantOut: "task T999 not found",
		},
		{
			name:    "View with invalid format",
			args:    []string{"invalid"},
			wantErr: true,
			wantOut: "invalid task_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			cmd := &cobra.Command{
				Use:  "view",
				Args: cobra.ExactArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
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

					fmt.Fprintf(&buf, "Task T%d: %s\n", task.ID, task.Title)
					fmt.Fprintf(&buf, "Status: %s\n", task.Status)
					return nil
				},
			}

			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !strings.Contains(buf.String(), tt.wantOut) {
				t.Errorf("Output = %v, want output containing %v", buf.String(), tt.wantOut)
			}
		})
	}
}

func TestUpdateCommand(t *testing.T) {
	_, db, cleanup := setupTestCLI(t)
	defer cleanup()

	// Add a test task
	_, err := tasks.AddTask(db, "Test Task", nil)
	if err != nil {
		t.Fatalf("Failed to add test task: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		wantOut string
	}{
		{
			name:    "Update to valid status",
			args:    []string{"T1", "in-progress"},
			wantErr: false,
			wantOut: "Updated T1 status to in-progress",
		},
		{
			name:    "Update to invalid status",
			args:    []string{"T1", "invalid"},
			wantErr: true,
			wantOut: "invalid status",
		},
		{
			name:    "Update with invalid task format",
			args:    []string{"invalid", "completed"},
			wantErr: true,
			wantOut: "invalid task_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			cmd := &cobra.Command{
				Use:  "update",
				Args: cobra.ExactArgs(2),
				RunE: func(cmd *cobra.Command, args []string) error {
					var taskID int
					if _, err := fmt.Sscanf(args[0], "T%d", &taskID); err != nil {
						return fmt.Errorf("invalid task_id format: %s", args[0])
					}

					status := args[1]
					if err := tasks.UpdateTaskStatus(db, taskID, status); err != nil {
						return fmt.Errorf("failed to update task status: %w", err)
					}

					fmt.Fprintf(&buf, "Updated T%d status to %s\n", taskID, status)
					return nil
				},
			}

			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !strings.Contains(buf.String(), tt.wantOut) {
				t.Errorf("Output = %v, want output containing %v", buf.String(), tt.wantOut)
			}
		})
	}
}

func TestNextCommand(t *testing.T) {
	_, db, cleanup := setupTestCLI(t)
	defer cleanup()

	tests := []struct {
		name    string
		setup   func() error
		wantOut string
		wantErr bool
	}{
		{
			name:    "No ready tasks",
			setup:   func() error { return nil },
			wantOut: "No tasks ready for work",
			wantErr: false,
		},
		{
			name: "Task ready for work",
			setup: func() error {
				_, err := tasks.AddTask(db, "Test Task", nil)
				return err
			},
			wantOut: "Task T1: Test Task",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.setup(); err != nil {
				t.Fatalf("Setup failed: %v", err)
			}

			var buf bytes.Buffer

			cmd := &cobra.Command{
				Use: "next",
				RunE: func(cmd *cobra.Command, args []string) error {
					task, err := tasks.GetNextTask(db)
					if err != nil {
						return fmt.Errorf("failed to get next task: %w", err)
					}

					if task == nil {
						fmt.Fprintln(&buf, "No tasks ready for work")
						return nil
					}

					fmt.Fprintf(&buf, "Task T%d: %s\n", task.ID, task.Title)
					fmt.Fprintf(&buf, "Status: %s\n", task.Status)
					return nil
				},
			}

			err := cmd.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !strings.Contains(buf.String(), tt.wantOut) {
				t.Errorf("Output = %v, want output containing %v", buf.String(), tt.wantOut)
			}
		})
	}
}

func TestLogCommand(t *testing.T) {
	_, db, cleanup := setupTestCLI(t)
	defer cleanup()

	// Add a test task
	_, err := tasks.AddTask(db, "Test Task", nil)
	if err != nil {
		t.Fatalf("Failed to add test task: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		wantOut string
	}{
		{
			name:    "Add log to existing task",
			args:    []string{"T1", "Test log message"},
			wantErr: false,
			wantOut: "Added log to T1",
		},
		{
			name:    "Add log with invalid task format",
			args:    []string{"invalid", "Test log"},
			wantErr: true,
			wantOut: "invalid task_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			cmd := &cobra.Command{
				Use:  "log",
				Args: cobra.ExactArgs(2),
				RunE: func(cmd *cobra.Command, args []string) error {
					var taskID int
					if _, err := fmt.Sscanf(args[0], "T%d", &taskID); err != nil {
						return fmt.Errorf("invalid task_id format: %s", args[0])
					}

					message := args[1]
					if err := tasks.AddTaskLog(db, taskID, message); err != nil {
						return fmt.Errorf("failed to add task log: %w", err)
					}

					fmt.Fprintf(&buf, "Added log to T%d\n", taskID)
					return nil
				},
			}

			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !strings.Contains(buf.String(), tt.wantOut) {
				t.Errorf("Output = %v, want output containing %v", buf.String(), tt.wantOut)
			}
		})
	}
}

func TestReviewCommand(t *testing.T) {
	_, db, cleanup := setupTestCLI(t)
	defer cleanup()

	// Add a test task
	_, err := tasks.AddTask(db, "Test Task", nil)
	if err != nil {
		t.Fatalf("Failed to add test task: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		wantOut string
	}{
		{
			name:    "Create review without attachment",
			args:    []string{"T1", "Test review message"},
			wantErr: false,
			wantOut: "Created review for T1",
		},
		{
			name:    "Create review with attachment",
			args:    []string{"T1", "Test review message", "test.md"},
			wantErr: false,
			wantOut: "Created review for T1",
		},
		{
			name:    "Create review with invalid task format",
			args:    []string{"invalid", "Test review"},
			wantErr: true,
			wantOut: "invalid task_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			cmd := &cobra.Command{
				Use:  "review",
				Args: cobra.RangeArgs(2, 3),
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

					if err := tasks.CreateReview(db, taskID, message, attachment); err != nil {
						return fmt.Errorf("failed to create review: %w", err)
					}

					fmt.Fprintf(&buf, "Created review for T%d\n", taskID)
					return nil
				},
			}

			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !strings.Contains(buf.String(), tt.wantOut) {
				t.Errorf("Output = %v, want output containing %v", buf.String(), tt.wantOut)
			}
		})
	}
}

func TestDeleteCommand(t *testing.T) {
	_, db, cleanup := setupTestCLI(t)
	defer cleanup()

	// Add a test task
	_, err := tasks.AddTask(db, "Test Task", nil)
	if err != nil {
		t.Fatalf("Failed to add test task: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		wantErr bool
		wantOut string
	}{
		{
			name:    "Delete existing task",
			args:    []string{"T1"},
			wantErr: false,
			wantOut: "Deleted T1",
		},
		{
			name:    "Delete with invalid format",
			args:    []string{"invalid"},
			wantErr: true,
			wantOut: "invalid task_id format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			cmd := &cobra.Command{
				Use:  "delete",
				Args: cobra.ExactArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					var taskID int
					if _, err := fmt.Sscanf(args[0], "T%d", &taskID); err != nil {
						return fmt.Errorf("invalid task_id format: %s", args[0])
					}

					if err := tasks.DeleteTask(db, taskID); err != nil {
						return fmt.Errorf("failed to delete task: %w", err)
					}

					fmt.Fprintf(&buf, "Deleted T%d\n", taskID)
					return nil
				},
			}

			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !strings.Contains(buf.String(), tt.wantOut) {
				t.Errorf("Output = %v, want output containing %v", buf.String(), tt.wantOut)
			}
		})
	}
}
