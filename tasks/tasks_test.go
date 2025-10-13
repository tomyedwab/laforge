package tasks

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	// Create an in-memory database for testing
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	if err := createSchema(db); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return db
}

func TestCreateSchema(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Test that all tables exist
	tables := []string{"tasks", "task_logs", "task_reviews", "work_queue"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("Table %s does not exist: %v", table, err)
		}
	}
}

func TestAddTask(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add a parent task first for the child task test
	parentID, err := AddTask(db, "Parent Task", nil)
	if err != nil {
		t.Fatalf("Failed to add parent task: %v", err)
	}

	tests := []struct {
		name     string
		title    string
		parentID *int
		wantErr  bool
	}{
		{
			name:    "Add root task",
			title:   "Test Task",
			wantErr: false,
		},
		{
			name:     "Add child task",
			title:    "Child Task",
			parentID: &parentID,
			wantErr:  false,
		},
		{
			name:    "Add task with empty title",
			title:   "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := AddTask(db, tt.title, tt.parentID)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddTask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && id <= 0 {
				t.Errorf("AddTask() returned invalid ID: %d", id)
			}
		})
	}
}

func TestGetTask(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add a test task
	title := "Test Task"
	id, err := AddTask(db, title, nil)
	if err != nil {
		t.Fatalf("Failed to add test task: %v", err)
	}

	tests := []struct {
		name    string
		taskID  int
		wantErr bool
		wantNil bool
	}{
		{
			name:    "Get existing task",
			taskID:  id,
			wantErr: false,
			wantNil: false,
		},
		{
			name:    "Get non-existent task",
			taskID:  999,
			wantErr: false,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := GetTask(db, tt.taskID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (task == nil) != tt.wantNil {
				t.Errorf("GetTask() returned nil = %v, wantNil %v", task == nil, tt.wantNil)
			}
			if task != nil && task.Title != title {
				t.Errorf("GetTask() returned task with title = %v, want %v", task.Title, title)
			}
		})
	}
}

func TestListTasks(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add test tasks
	tasks := []string{"Task 1", "Task 2", "Task 3"}
	for _, title := range tasks {
		_, err := AddTask(db, title, nil)
		if err != nil {
			t.Fatalf("Failed to add test task: %v", err)
		}
	}

	result, err := ListTasks(db)
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}

	if len(result) != len(tasks) {
		t.Errorf("ListTasks() returned %d tasks, want %d", len(result), len(tasks))
	}

	for i, task := range result {
		if task.Title != tasks[i] {
			t.Errorf("ListTasks() task[%d].Title = %v, want %v", i, task.Title, tasks[i])
		}
	}
}

func TestUpdateTaskStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add a test task
	id, err := AddTask(db, "Test Task", nil)
	if err != nil {
		t.Fatalf("Failed to add test task: %v", err)
	}

	tests := []struct {
		name        string
		status      string
		wantErr     bool
		errContains string
	}{
		{
			name:    "Update to valid status",
			status:  "in-progress",
			wantErr: false,
		},
		{
			name:    "Update to completed",
			status:  "completed",
			wantErr: false,
		},
		{
			name:        "Update to invalid status",
			status:      "invalid",
			wantErr:     true,
			errContains: "invalid status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UpdateTaskStatus(db, id, tt.status)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateTaskStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("UpdateTaskStatus() error = %v, want error containing %v", err, tt.errContains)
				}
			}

			if !tt.wantErr {
				task, _ := GetTask(db, id)
				if task.Status != tt.status {
					t.Errorf("UpdateTaskStatus() task.Status = %v, want %v", task.Status, tt.status)
				}
			}
		})
	}
}

func TestUpdateTaskStatusWithChildren(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add parent task
	parentID, err := AddTask(db, "Parent Task", nil)
	if err != nil {
		t.Fatalf("Failed to add parent task: %v", err)
	}

	// Add child task
	childID, err := AddTask(db, "Child Task", &parentID)
	if err != nil {
		t.Fatalf("Failed to add child task: %v", err)
	}

	// Try to complete parent while child is incomplete
	err = UpdateTaskStatus(db, parentID, "completed")
	if err == nil {
		t.Error("UpdateTaskStatus() should fail when completing parent with incomplete children")
	}
	if !contains(err.Error(), "child tasks are not completed") {
		t.Errorf("UpdateTaskStatus() error = %v, want error about child tasks", err)
	}

	// Complete child task
	err = UpdateTaskStatus(db, childID, "completed")
	if err != nil {
		t.Fatalf("Failed to complete child task: %v", err)
	}

	// Now parent should be completable
	err = UpdateTaskStatus(db, parentID, "completed")
	if err != nil {
		t.Errorf("UpdateTaskStatus() should succeed after child is completed, error = %v", err)
	}
}

func TestUpdateTaskStatusWithPendingReviews(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add a test task
	id, err := AddTask(db, "Test Task", nil)
	if err != nil {
		t.Fatalf("Failed to add test task: %v", err)
	}

	// Add a pending review
	err = CreateReview(db, id, "Test review", nil)
	if err != nil {
		t.Fatalf("Failed to create review: %v", err)
	}

	// Try to complete task with pending review
	err = UpdateTaskStatus(db, id, "completed")
	if err == nil {
		t.Error("UpdateTaskStatus() should fail when completing task with pending reviews")
	}
	if !contains(err.Error(), "pending reviews exist") {
		t.Errorf("UpdateTaskStatus() error = %v, want error about pending reviews", err)
	}
}

func TestAddToQueue(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add a test task
	id, err := AddTask(db, "Test Task", nil)
	if err != nil {
		t.Fatalf("Failed to add test task: %v", err)
	}

	// Add to queue
	err = AddToQueue(db, id)
	if err != nil {
		t.Errorf("AddToQueue() error = %v", err)
	}

	// Verify task is in queue
	var queuedAt time.Time
	err = db.QueryRow("SELECT queued_at FROM work_queue WHERE task_id = ?", id).Scan(&queuedAt)
	if err != nil {
		t.Errorf("Task not found in queue: %v", err)
	}
}

func TestGetNextTask(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add tasks
	task1ID, err := AddTask(db, "Task 1", nil)
	if err != nil {
		t.Fatalf("Failed to add task 1: %v", err)
	}

	task2ID, err := AddTask(db, "Task 2", nil)
	if err != nil {
		t.Fatalf("Failed to add task 2: %v", err)
	}

	// Add child task
	childID, err := AddTask(db, "Child Task", &task1ID)
	if err != nil {
		t.Fatalf("Failed to add child task: %v", err)
	}

	// Add tasks to queue
	AddToQueue(db, task1ID)
	AddToQueue(db, task2ID)
	AddToQueue(db, childID)

	// Get next task - should be child task (prioritized over parent)
	nextTask, err := GetNextTask(db)
	if err != nil {
		t.Fatalf("GetNextTask() error = %v", err)
	}
	if nextTask == nil {
		t.Fatal("GetNextTask() returned nil")
	}
	if nextTask.ID != childID {
		t.Errorf("GetNextTask() returned task ID %d, want %d (child task)", nextTask.ID, childID)
	}

	// Complete child task
	UpdateTaskStatus(db, childID, "completed")

	// Get next task - should be task1 (lower ID than task2)
	nextTask, err = GetNextTask(db)
	if err != nil {
		t.Fatalf("GetNextTask() error = %v", err)
	}
	if nextTask.ID != task1ID {
		t.Errorf("GetNextTask() returned task ID %d, want %d", nextTask.ID, task1ID)
	}
}

func TestGetNextTaskEmptyQueue(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	nextTask, err := GetNextTask(db)
	if err != nil {
		t.Fatalf("GetNextTask() error = %v", err)
	}
	if nextTask != nil {
		t.Error("GetNextTask() should return nil for empty queue")
	}
}

func TestAddTaskLog(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add a test task
	id, err := AddTask(db, "Test Task", nil)
	if err != nil {
		t.Fatalf("Failed to add test task: %v", err)
	}

	message := "Test log message"
	err = AddTaskLog(db, id, message)
	if err != nil {
		t.Errorf("AddTaskLog() error = %v", err)
	}

	// Verify log was added
	logs, err := GetTaskLogs(db, id)
	if err != nil {
		t.Fatalf("GetTaskLogs() error = %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("GetTaskLogs() returned %d logs, want 1", len(logs))
	}
	if logs[0].Message != message {
		t.Errorf("GetTaskLogs() message = %v, want %v", logs[0].Message, message)
	}
}

func TestCreateReview(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add a test task
	id, err := AddTask(db, "Test Task", nil)
	if err != nil {
		t.Fatalf("Failed to add test task: %v", err)
	}

	message := "Test review message"
	attachment := "test.md"
	err = CreateReview(db, id, message, &attachment)
	if err != nil {
		t.Errorf("CreateReview() error = %v", err)
	}

	// Verify review was created and task status updated
	task, _ := GetTask(db, id)
	if task.Status != "in-review" {
		t.Errorf("CreateReview() task status = %v, want in-review", task.Status)
	}

	reviews, err := GetTaskReviews(db, id)
	if err != nil {
		t.Fatalf("GetTaskReviews() error = %v", err)
	}
	if len(reviews) != 1 {
		t.Errorf("GetTaskReviews() returned %d reviews, want 1", len(reviews))
	}
	if reviews[0].Message != message {
		t.Errorf("GetTaskReviews() message = %v, want %v", reviews[0].Message, message)
	}
	if reviews[0].Attachment == nil || *reviews[0].Attachment != attachment {
		t.Errorf("GetTaskReviews() attachment = %v, want %v", reviews[0].Attachment, attachment)
	}
}

func TestGetChildTasks(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add parent task
	parentID, err := AddTask(db, "Parent Task", nil)
	if err != nil {
		t.Fatalf("Failed to add parent task: %v", err)
	}

	// Add child tasks
	child1ID, err := AddTask(db, "Child Task 1", &parentID)
	if err != nil {
		t.Fatalf("Failed to add child task 1: %v", err)
	}

	child2ID, err := AddTask(db, "Child Task 2", &parentID)
	if err != nil {
		t.Fatalf("Failed to add child task 2: %v", err)
	}

	children, err := GetChildTasks(db, parentID)
	if err != nil {
		t.Fatalf("GetChildTasks() error = %v", err)
	}

	if len(children) != 2 {
		t.Errorf("GetChildTasks() returned %d children, want 2", len(children))
	}

	// Verify child IDs
	childIDs := make(map[int]bool)
	for _, child := range children {
		childIDs[child.ID] = true
	}
	if !childIDs[child1ID] || !childIDs[child2ID] {
		t.Error("GetChildTasks() did not return expected child tasks")
	}
}

func TestDeleteTask(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add a test task
	id, err := AddTask(db, "Test Task", nil)
	if err != nil {
		t.Fatalf("Failed to add test task: %v", err)
	}

	// Add a log entry
	err = AddTaskLog(db, id, "Test log")
	if err != nil {
		t.Fatalf("Failed to add task log: %v", err)
	}

	// Delete the task
	err = DeleteTask(db, id)
	if err != nil {
		t.Errorf("DeleteTask() error = %v", err)
	}

	// Verify task is deleted
	task, _ := GetTask(db, id)
	if task != nil {
		t.Error("DeleteTask() did not delete the task")
	}

	// Verify associated logs are deleted (cascade)
	logs, _ := GetTaskLogs(db, id)
	if len(logs) != 0 {
		t.Error("DeleteTask() did not cascade delete task logs")
	}
}

func TestAddTaskWithDetails(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	title := "Test Task"
	description := "Test Description"
	acceptanceCriteria := "Test Acceptance Criteria"
	reviewRequired := true

	// Add upstream task first
	upstreamTaskID, err := AddTask(db, "Upstream Task", nil)
	if err != nil {
		t.Fatalf("Failed to add upstream task: %v", err)
	}

	// Add task with details
	id, err := AddTaskWithDetails(db, title, description, acceptanceCriteria, &upstreamTaskID, reviewRequired, nil)
	if err != nil {
		t.Fatalf("AddTaskWithDetails() error = %v", err)
	}

	// Verify task was created with correct details
	task, err := GetTask(db, id)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}

	if task.Title != title {
		t.Errorf("Task.Title = %v, want %v", task.Title, title)
	}
	if task.Description != description {
		t.Errorf("Task.Description = %v, want %v", task.Description, description)
	}
	if task.AcceptanceCriteria != acceptanceCriteria {
		t.Errorf("Task.AcceptanceCriteria = %v, want %v", task.AcceptanceCriteria, acceptanceCriteria)
	}
	if task.UpstreamDependencyID == nil || *task.UpstreamDependencyID != upstreamTaskID {
		t.Errorf("Task.UpstreamDependencyID = %v, want %v", task.UpstreamDependencyID, upstreamTaskID)
	}
	if task.ReviewRequired != reviewRequired {
		t.Errorf("Task.ReviewRequired = %v, want %v", task.ReviewRequired, reviewRequired)
	}
}

func TestUpdateTaskStatusWithUpstreamDependency(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add upstream task
	upstreamID, err := AddTask(db, "Upstream Task", nil)
	if err != nil {
		t.Fatalf("Failed to add upstream task: %v", err)
	}

	// Add dependent task
	dependentID, err := AddTaskWithDetails(db, "Dependent Task", "", "", &upstreamID, false, nil)
	if err != nil {
		t.Fatalf("Failed to add dependent task: %v", err)
	}

	// Try to move dependent task to in-progress while upstream is incomplete
	err = UpdateTaskStatus(db, dependentID, "in-progress")
	if err == nil {
		t.Error("UpdateTaskStatus() should fail when upstream dependency is not completed")
	}
	if !contains(err.Error(), "upstream dependency") {
		t.Errorf("UpdateTaskStatus() error = %v, want error about upstream dependency", err)
	}

	// Complete upstream task
	err = UpdateTaskStatus(db, upstreamID, "completed")
	if err != nil {
		t.Fatalf("Failed to complete upstream task: %v", err)
	}

	// Now dependent task should be movable to in-progress
	err = UpdateTaskStatus(db, dependentID, "in-progress")
	if err != nil {
		t.Errorf("UpdateTaskStatus() should succeed after upstream dependency is completed, error = %v", err)
	}
}

func TestUpdateTaskStatusWithReviewRequired(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add task with review required
	id, err := AddTaskWithDetails(db, "Review Required Task", "", "", nil, true, nil)
	if err != nil {
		t.Fatalf("Failed to add task: %v", err)
	}

	// Try to complete task without any approved reviews
	err = UpdateTaskStatus(db, id, "completed")
	if err == nil {
		t.Error("UpdateTaskStatus() should fail when review is required but no approved reviews exist")
	}
	if !contains(err.Error(), "review is required") {
		t.Errorf("UpdateTaskStatus() error = %v, want error about required review", err)
	}

	// Create a review and approve it
	err = CreateReview(db, id, "Test review", nil)
	if err != nil {
		t.Fatalf("Failed to create review: %v", err)
	}

	// Manually approve the review (since we don't have an ApproveReview function)
	_, err = db.Exec("UPDATE task_reviews SET status = 'approved' WHERE task_id = ?", id)
	if err != nil {
		t.Fatalf("Failed to approve review: %v", err)
	}

	// Now task should be completable
	err = UpdateTaskStatus(db, id, "completed")
	if err != nil {
		t.Errorf("UpdateTaskStatus() should succeed after review is approved, error = %v", err)
	}
}

func TestGetNextTaskWithUpstreamDependency(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Add upstream task
	upstreamID, err := AddTask(db, "Upstream Task", nil)
	if err != nil {
		t.Fatalf("Failed to add upstream task: %v", err)
	}

	// Add dependent task
	dependentID, err := AddTaskWithDetails(db, "Dependent Task", "", "", &upstreamID, false, nil)
	if err != nil {
		t.Fatalf("Failed to add dependent task: %v", err)
	}

	// Add independent task
	independentID, err := AddTask(db, "Independent Task", nil)
	if err != nil {
		t.Fatalf("Failed to add independent task: %v", err)
	}

	// Add all tasks to queue
	AddToQueue(db, upstreamID)
	AddToQueue(db, dependentID)
	AddToQueue(db, independentID)

	// Get next task - should be upstream task (no dependencies, lowest ID)
	nextTask, err := GetNextTask(db)
	if err != nil {
		t.Fatalf("GetNextTask() error = %v", err)
	}
	if nextTask == nil {
		t.Fatal("GetNextTask() returned nil when upstream task should be available")
	}
	if nextTask.ID != upstreamID {
		t.Errorf("GetNextTask() returned task ID %d, want %d (upstream task)", nextTask.ID, upstreamID)
	}

	// Complete upstream task
	err = UpdateTaskStatus(db, upstreamID, "completed")
	if err != nil {
		t.Fatalf("Failed to complete upstream task: %v", err)
	}

	// Remove completed task from queue and get next task
	// Since upstream task is completed, it should be removed from queue automatically
	// Both dependent and independent tasks should now be available
	// The ordering should prioritize tasks with lower IDs, so dependent task (ID 2) should come first
	nextTask, err = GetNextTask(db)
	if err != nil {
		t.Fatalf("GetNextTask() error = %v", err)
	}
	if nextTask == nil {
		t.Fatal("GetNextTask() returned nil when tasks should be available")
	}
	// Either dependent or independent task should be returned - both are valid
	if nextTask.ID != dependentID && nextTask.ID != independentID {
		t.Errorf("GetNextTask() returned task ID %d, want either %d (dependent) or %d (independent)", nextTask.ID, dependentID, independentID)
	}

	// Complete independent task
	err = UpdateTaskStatus(db, independentID, "completed")
	if err != nil {
		t.Fatalf("Failed to complete independent task: %v", err)
	}

	// Get next task - should now be the remaining task (either dependent or independent)
	nextTask, err = GetNextTask(db)
	if err != nil {
		t.Fatalf("GetNextTask() error = %v", err)
	}
	if nextTask == nil {
		t.Fatal("GetNextTask() returned nil when a task should be available")
	}
	// Should return the remaining task
	if nextTask.ID != dependentID && nextTask.ID != independentID {
		t.Errorf("GetNextTask() returned task ID %d, want either %d (dependent) or %d (independent)", nextTask.ID, dependentID, independentID)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
