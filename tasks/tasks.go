package tasks

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Task struct {
	ID                   int
	Title                string
	Description          string
	AcceptanceCriteria   string
	UpstreamDependencyID *int
	ReviewRequired       bool
	ParentID             *int
	Status               string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type TaskLog struct {
	ID        int
	TaskID    int
	Message   string
	CreatedAt time.Time
}

type TaskReview struct {
	ID         int
	TaskID     int
	Message    string
	Attachment *string
	Status     string
	Feedback   *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func InitDB() (*sql.DB, error) {
	// Try to use environment variable for database path, fallback to default
	dbPath := os.Getenv("TASKS_DB_PATH")
	if dbPath == "" {
		dbPath = "/state/tasks.db"
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if err := createSchema(db); err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return db, nil
}

func createSchema(db *sql.DB) error {
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
	);

	CREATE TABLE IF NOT EXISTS work_queue (
		task_id INTEGER PRIMARY KEY,
		queued_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
	);`

	_, err := db.Exec(schema)
	return err
}

func AddTask(db *sql.DB, title string, parentID *int) (int, error) {
	result, err := db.Exec("INSERT INTO tasks (title, parent_id) VALUES (?, ?)", title, parentID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert task: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return int(id), nil
}

func AddTaskWithDetails(db *sql.DB, title string, description string, acceptanceCriteria string, upstreamDependencyID *int, reviewRequired bool, parentID *int) (int, error) {
	result, err := db.Exec("INSERT INTO tasks (title, description, acceptance_criteria, upstream_dependency_id, review_required, parent_id) VALUES (?, ?, ?, ?, ?, ?)",
		title, description, acceptanceCriteria, upstreamDependencyID, reviewRequired, parentID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert task: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return int(id), nil
}

func GetTask(db *sql.DB, taskID int) (*Task, error) {
	var task Task
	err := db.QueryRow("SELECT id, title, description, acceptance_criteria, upstream_dependency_id, review_required, parent_id, status, created_at, updated_at FROM tasks WHERE id = ?", taskID).
		Scan(&task.ID, &task.Title, &task.Description, &task.AcceptanceCriteria, &task.UpstreamDependencyID, &task.ReviewRequired, &task.ParentID, &task.Status, &task.CreatedAt, &task.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query task: %w", err)
	}

	return &task, nil
}

func ListTasks(db *sql.DB) ([]Task, error) {
	rows, err := db.Query("SELECT id, title, description, acceptance_criteria, upstream_dependency_id, review_required, parent_id, status, created_at, updated_at FROM tasks ORDER BY id")
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.Title, &task.Description, &task.AcceptanceCriteria, &task.UpstreamDependencyID, &task.ReviewRequired, &task.ParentID, &task.Status, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func UpdateTaskStatus(db *sql.DB, taskID int, status string) error {
	validStatuses := map[string]bool{
		"todo":        true,
		"in-progress": true,
		"in-review":   true,
		"completed":   true,
	}

	if !validStatuses[status] {
		return fmt.Errorf("invalid status: %s", status)
	}

	// Get the current task to check its properties
	task, err := GetTask(db, taskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("task not found: T%d", taskID)
	}

	// Check upstream dependency for in-progress and completed statuses
	if status == "in-progress" || status == "completed" {
		if task.UpstreamDependencyID != nil {
			var upstreamStatus string
			err := db.QueryRow("SELECT status FROM tasks WHERE id = ?", *task.UpstreamDependencyID).Scan(&upstreamStatus)
			if err != nil {
				return fmt.Errorf("failed to check upstream dependency: %w", err)
			}
			if upstreamStatus != "completed" {
				return fmt.Errorf("cannot move task T%d to '%s': upstream dependency T%d is not completed", taskID, status, *task.UpstreamDependencyID)
			}
		}
	}

	// Check if task has incomplete child tasks when trying to complete
	if status == "completed" {
		var incompleteChildren int
		err := db.QueryRow("SELECT COUNT(*) FROM tasks WHERE parent_id = ? AND status != 'completed'", taskID).Scan(&incompleteChildren)
		if err != nil {
			return fmt.Errorf("failed to check child tasks: %w", err)
		}
		if incompleteChildren > 0 {
			return fmt.Errorf("cannot complete task T%d: %d child tasks are not completed", taskID, incompleteChildren)
		}

		// Check if task has pending reviews
		var pendingReviews int
		err = db.QueryRow("SELECT COUNT(*) FROM task_reviews WHERE task_id = ? AND status = 'pending'", taskID).Scan(&pendingReviews)
		if err != nil {
			return fmt.Errorf("failed to check pending reviews: %w", err)
		}
		if pendingReviews > 0 {
			return fmt.Errorf("cannot complete task T%d: %d pending reviews exist", taskID, pendingReviews)
		}

		// Check if review is required but no approved reviews exist
		if task.ReviewRequired {
			var approvedReviews int
			err = db.QueryRow("SELECT COUNT(*) FROM task_reviews WHERE task_id = ? AND status = 'approved'", taskID).Scan(&approvedReviews)
			if err != nil {
				return fmt.Errorf("failed to check approved reviews: %w", err)
			}
			if approvedReviews == 0 {
				return fmt.Errorf("cannot complete task T%d: review is required but no approved reviews exist", taskID)
			}
		}
	}

	_, err = db.Exec("UPDATE tasks SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", status, taskID)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	if status == "completed" {
		_, err = db.Exec("DELETE FROM work_queue WHERE task_id = ?", taskID)
		if err != nil {
			return fmt.Errorf("failed to remove from queue: %w", err)
		}
	}

	return nil
}

func AddToQueue(db *sql.DB, taskID int) error {
	_, err := db.Exec("INSERT OR REPLACE INTO work_queue (task_id) VALUES (?)", taskID)
	return err
}

func GetNextTask(db *sql.DB) (*Task, error) {
	// First, get all candidate tasks from the queue
	query := `
		SELECT t.id, t.title, t.description, t.acceptance_criteria, t.upstream_dependency_id, t.review_required, t.parent_id, t.status, t.created_at, t.updated_at
		FROM tasks t
		JOIN work_queue w ON t.id = w.task_id
		WHERE t.status != 'completed'
		ORDER BY 
			CASE WHEN t.parent_id IS NULL THEN 1 ELSE 0 END,
			t.id`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get next task: %w", err)
	}
	defer rows.Close()

	var candidateTasks []Task
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.Title, &task.Description, &task.AcceptanceCriteria, &task.UpstreamDependencyID, &task.ReviewRequired, &task.ParentID, &task.Status, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		candidateTasks = append(candidateTasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate tasks: %w", err)
	}

	// Now check upstream dependencies for each candidate task
	for _, task := range candidateTasks {
		// Check if upstream dependency is completed
		if task.UpstreamDependencyID != nil {
			var upstreamStatus string
			err := db.QueryRow("SELECT status FROM tasks WHERE id = ?", *task.UpstreamDependencyID).Scan(&upstreamStatus)
			if err != nil {
				continue // Skip this task if we can't check upstream dependency
			}
			if upstreamStatus != "completed" {
				continue // Skip this task if upstream dependency is not completed
			}
		}

		return &task, nil
	}

	return nil, nil
}

func AddTaskLog(db *sql.DB, taskID int, message string) error {
	_, err := db.Exec("INSERT INTO task_logs (task_id, message) VALUES (?, ?)", taskID, message)
	return err
}

func GetTaskLogs(db *sql.DB, taskID int) ([]TaskLog, error) {
	rows, err := db.Query("SELECT id, task_id, message, created_at FROM task_logs WHERE task_id = ? ORDER BY created_at DESC", taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query task logs: %w", err)
	}
	defer rows.Close()

	var logs []TaskLog
	for rows.Next() {
		var log TaskLog
		if err := rows.Scan(&log.ID, &log.TaskID, &log.Message, &log.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan task log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}

func CreateReview(db *sql.DB, taskID int, message string, attachment *string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec("INSERT INTO task_reviews (task_id, message, attachment) VALUES (?, ?, ?)", taskID, message, attachment)
	if err != nil {
		return fmt.Errorf("failed to create review: %w", err)
	}

	_, err = tx.Exec("UPDATE tasks SET status = 'in-review', updated_at = CURRENT_TIMESTAMP WHERE id = ?", taskID)
	if err != nil {
		return fmt.Errorf("failed to update task status: %w", err)
	}

	return tx.Commit()
}

func GetTaskReviews(db *sql.DB, taskID int) ([]TaskReview, error) {
	rows, err := db.Query("SELECT id, task_id, message, attachment, status, feedback, created_at, updated_at FROM task_reviews WHERE task_id = ? ORDER BY created_at DESC", taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to query task reviews: %w", err)
	}
	defer rows.Close()

	var reviews []TaskReview
	for rows.Next() {
		var review TaskReview
		if err := rows.Scan(&review.ID, &review.TaskID, &review.Message, &review.Attachment, &review.Status, &review.Feedback, &review.CreatedAt, &review.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan task review: %w", err)
		}
		reviews = append(reviews, review)
	}

	return reviews, nil
}

func GetChildTasks(db *sql.DB, parentID int) ([]Task, error) {
	rows, err := db.Query("SELECT id, title, description, acceptance_criteria, upstream_dependency_id, review_required, parent_id, status, created_at, updated_at FROM tasks WHERE parent_id = ? ORDER BY id", parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query child tasks: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var task Task
		if err := rows.Scan(&task.ID, &task.Title, &task.Description, &task.AcceptanceCriteria, &task.UpstreamDependencyID, &task.ReviewRequired, &task.ParentID, &task.Status, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan child task: %w", err)
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func DeleteTask(db *sql.DB, taskID int) error {
	_, err := db.Exec("DELETE FROM tasks WHERE id = ?", taskID)
	return err
}
