package tasks

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v3"
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

	if status == "in-review" && task.Status != "in-review" {
		return fmt.Errorf("cannot set task to in-review directly; add a review request instead")
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

	return nil
}

func GetNextTask(db *sql.DB) (*Task, error) {
	// Get all candidate tasks that are ready for work based on their status
	// A task is ready if:
	// - Status is 'todo', 'in-progress', or 'in-review' (with no pending reviews)
	// - All upstream dependencies are completed
	query := `
		SELECT t.id, t.title, t.description, t.acceptance_criteria, t.upstream_dependency_id, t.review_required, t.parent_id, t.status, t.created_at, t.updated_at
		FROM tasks t
		WHERE t.status IN ('todo', 'in-progress', 'in-review')
		ORDER BY
			CASE WHEN t.parent_id IS NULL THEN 0 ELSE 1 END,
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

	// Now check each candidate task to see if it's ready
	for _, task := range candidateTasks {
		// For in-review tasks, check that there are no pending reviews
		if task.Status == "in-review" {
			var pendingReviews int
			err := db.QueryRow("SELECT COUNT(*) FROM task_reviews WHERE task_id = ? AND status = 'pending'", task.ID).Scan(&pendingReviews)
			if err != nil {
				continue // Skip this task if we can't check reviews
			}
			if pendingReviews > 0 {
				continue // Skip this task if it has pending reviews
			}
		}

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

		// Check if task has incomplete child tasks (epics should not be worked on until children are done)
		var incompleteChildren int
		err := db.QueryRow("SELECT COUNT(*) FROM tasks WHERE parent_id = ? AND status != 'completed'", task.ID).Scan(&incompleteChildren)
		if err != nil {
			continue // Skip this task if we can't check child tasks
		}
		if incompleteChildren > 0 {
			continue // Skip this task if it has incomplete child tasks
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

func GetPendingReviews(db *sql.DB) ([]TaskReview, error) {
	rows, err := db.Query("SELECT id, task_id, message, attachment, status, feedback, created_at, updated_at FROM task_reviews WHERE status = 'pending' ORDER BY created_at ASC")
	if err != nil {
		return nil, fmt.Errorf("failed to query pending reviews: %w", err)
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

func UpdateReview(db *sql.DB, reviewID int, status string, feedback *string) error {
	validStatuses := map[string]bool{
		"pending":  true,
		"approved": true,
		"rejected": true,
	}

	if !validStatuses[status] {
		return fmt.Errorf("invalid review status: %s", status)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update the review
	_, err = tx.Exec("UPDATE task_reviews SET status = ?, feedback = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", status, feedback, reviewID)
	if err != nil {
		return fmt.Errorf("failed to update review: %w", err)
	}

	// If this was the last pending review for a task that's in-review status,
	// we might need to update the task status
	// Get the task_id for this review
	var taskID int
	err = tx.QueryRow("SELECT task_id FROM task_reviews WHERE id = ?", reviewID).Scan(&taskID)
	if err != nil {
		return fmt.Errorf("failed to get task_id for review: %w", err)
	}

	// Check if there are any remaining pending reviews for this task
	var pendingCount int
	err = tx.QueryRow("SELECT COUNT(*) FROM task_reviews WHERE task_id = ? AND status = 'pending'", taskID).Scan(&pendingCount)
	if err != nil {
		return fmt.Errorf("failed to count pending reviews: %w", err)
	}

	// If no more pending reviews and task is in-review, update task status to in-progress
	// so it can be worked on again
	if pendingCount == 0 {
		var taskStatus string
		err = tx.QueryRow("SELECT status FROM tasks WHERE id = ?", taskID).Scan(&taskStatus)
		if err != nil {
			return fmt.Errorf("failed to get task status: %w", err)
		}

		if taskStatus == "in-review" {
			_, err = tx.Exec("UPDATE tasks SET status = 'in-progress', updated_at = CURRENT_TIMESTAMP WHERE id = ?", taskID)
			if err != nil {
				return fmt.Errorf("failed to update task status: %w", err)
			}
		}
	}

	return tx.Commit()
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

// YAMLTask represents a task in the YAML import format
type YAMLTask struct {
	ID                   interface{} `yaml:"id"` // int, string ("T15"), or string ("new-*"/"tmp-*")
	Title                string      `yaml:"title"`
	Description          string      `yaml:"description"`
	AcceptanceCriteria   string      `yaml:"acceptance_criteria"`
	UpstreamDependencyID interface{} `yaml:"upstream_dependency_id"` // int, string ("T15"), or string ("new-*"/"tmp-*")
	ReviewRequired       bool        `yaml:"review_required"`
	ParentID             interface{} `yaml:"parent_id"` // int, string ("T15"), or string ("new-*"/"tmp-*")
	Status               string      `yaml:"status"`
}

// YAMLTaskLog represents a task log entry in the YAML import format
type YAMLTaskLog struct {
	TaskID  interface{} `yaml:"task_id"` // int, string ("T15"), or string ("new-*"/"tmp-*")
	Message string      `yaml:"message"`
}

// YAMLTaskReview represents a task review in the YAML import format
type YAMLTaskReview struct {
	TaskID     interface{} `yaml:"task_id"` // int, string ("T15"), or string ("new-*"/"tmp-*")
	Message    string      `yaml:"message"`
	Attachment *string     `yaml:"attachment"`
	Status     string      `yaml:"status"`
	Feedback   *string     `yaml:"feedback"`
}

// YAMLProject represents a project with nested tasks
type YAMLProject struct {
	Name  string     `yaml:"name"`
	Tasks []YAMLTask `yaml:"tasks"`
}

// YAMLTaskSpec represents the complete YAML task specification format
type YAMLTaskSpec struct {
	Tasks            []YAMLTask       `yaml:"tasks"`
	Projects         []YAMLProject    `yaml:"projects"`
	WorkflowExamples []YAMLTask       `yaml:"workflow_examples"`
	TaskLogs         []YAMLTaskLog    `yaml:"task_logs"`
	TaskReviews      []YAMLTaskReview `yaml:"task_reviews"`
}

// ImportTasksFromYAML imports tasks from a YAML file
func ImportTasksFromYAML(db *sql.DB, yamlPath string) error {
	// Read the YAML file
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return fmt.Errorf("failed to read YAML file: %w", err)
	}

	// Parse the YAML
	var spec YAMLTaskSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Validate and import tasks, get ID mapping
	yamlIDToDBID, err := importTasks(tx, spec.Tasks)
	if err != nil {
		return fmt.Errorf("failed to import tasks: %w", err)
	}

	// Import task logs
	if err := importTaskLogs(tx, spec.TaskLogs, yamlIDToDBID); err != nil {
		return fmt.Errorf("failed to import task logs: %w", err)
	}

	// Import task reviews
	if err := importTaskReviews(tx, spec.TaskReviews, yamlIDToDBID); err != nil {
		return fmt.Errorf("failed to import task reviews: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// parsedTask represents a task with its parsed ID information
type parsedTask struct {
	task       YAMLTask
	idResult   *TaskIDResult
	parentID   *TaskIDResult
	upstreamID *TaskIDResult
}

// importTasks validates and imports tasks, handling both new and existing tasks
// Returns a map from all task identifiers (both YAML local IDs and DB IDs) to database IDs
func importTasks(tx *sql.Tx, tasks []YAMLTask) (map[string]int, error) {
	if len(tasks) == 0 {
		return make(map[string]int), nil
	}

	validStatuses := map[string]bool{
		"todo":        true,
		"in-progress": true,
		"in-review":   true,
		"completed":   true,
	}

	// Get all existing task IDs from database
	existingTaskIDs := make(map[int]bool)
	rows, err := tx.Query("SELECT id FROM tasks")
	if err != nil {
		return nil, fmt.Errorf("failed to query existing task IDs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan task ID: %w", err)
		}
		existingTaskIDs[id] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate existing task IDs: %w", err)
	}

	// Phase 1: Parse all task IDs and categorize tasks
	var parsedTasks []parsedTask
	existingDBIDs := make(map[int]bool)
	localRefs := make(map[string]bool)

	for i, task := range tasks {
		// Parse task ID
		idResult, err := parseTaskID(task.ID, existingTaskIDs)
		if err != nil {
			return nil, fmt.Errorf("task %d: %w", i, err)
		}

		// Parse parent ID
		var parentID *TaskIDResult
		if task.ParentID != nil {
			parentID, err = parseTaskID(task.ParentID, existingTaskIDs)
			if err != nil {
				return nil, fmt.Errorf("task %d parent_id: %w", i, err)
			}
		}

		// Parse upstream dependency ID
		var upstreamID *TaskIDResult
		if task.UpstreamDependencyID != nil {
			upstreamID, err = parseTaskID(task.UpstreamDependencyID, existingTaskIDs)
			if err != nil {
				return nil, fmt.Errorf("task %d upstream_dependency_id: %w", i, err)
			}
		}

		// Validate task has title
		if task.Title == "" {
			return nil, fmt.Errorf("task %d has empty title", i)
		}

		// Validate status
		status := task.Status
		if status == "" {
			status = "todo"
		}
		if !validStatuses[status] {
			return nil, fmt.Errorf("task %d has invalid status: %s", i, status)
		}

		parsedTasks = append(parsedTasks, parsedTask{
			task:       task,
			idResult:   idResult,
			parentID:   parentID,
			upstreamID: upstreamID,
		})

		// Track referenced IDs
		if idResult.IsExisting {
			existingDBIDs[idResult.DBID] = true
		} else if idResult.LocalID != "" {
			localRefs[idResult.LocalID] = true
		}

		if parentID != nil {
			if parentID.IsExisting {
				existingDBIDs[parentID.DBID] = true
			} else if parentID.LocalID != "" {
				localRefs[parentID.LocalID] = true
			}
		}

		if upstreamID != nil {
			if upstreamID.IsExisting {
				existingDBIDs[upstreamID.DBID] = true
			} else if upstreamID.LocalID != "" {
				localRefs[upstreamID.LocalID] = true
			}
		}
	}

	// Phase 2: Validate all referenced existing DB IDs exist
	for dbID := range existingDBIDs {
		var exists bool
		err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM tasks WHERE id = ?)", dbID).Scan(&exists)
		if err != nil {
			return nil, fmt.Errorf("failed to check if task T%d exists: %w", dbID, err)
		}
		if !exists {
			return nil, fmt.Errorf("referenced task T%d does not exist in database", dbID)
		}
	}

	// Phase 3: Build initial ID map with existing DB IDs (they map to themselves)
	idMap := make(map[string]int)
	for dbID := range existingDBIDs {
		idMap[fmt.Sprintf("T%d", dbID)] = dbID
		idMap[fmt.Sprintf("%d", dbID)] = dbID
	}

	// Phase 4: Separate tasks into updates and creates
	var tasksToUpdate []parsedTask
	var tasksToCreate []parsedTask

	for _, pt := range parsedTasks {
		if pt.idResult.IsExisting {
			tasksToUpdate = append(tasksToUpdate, pt)
		} else {
			tasksToCreate = append(tasksToCreate, pt)
		}
	}

	// Phase 5: Process updates
	for _, pt := range tasksToUpdate {
		status := pt.task.Status
		if status == "" {
			status = "todo"
		}

		// Resolve parent and upstream IDs
		var parentID *int
		var upstreamID *int

		if pt.parentID != nil {
			resolvedID, err := resolveTaskReference(pt.parentID, idMap)
			if err != nil {
				return nil, fmt.Errorf("task T%d parent_id: %w", pt.idResult.DBID, err)
			}
			parentID = resolvedID
		}

		if pt.upstreamID != nil {
			resolvedID, err := resolveTaskReference(pt.upstreamID, idMap)
			if err != nil {
				return nil, fmt.Errorf("task T%d upstream_dependency_id: %w", pt.idResult.DBID, err)
			}
			upstreamID = resolvedID
		}

		// Update the existing task
		_, err := tx.Exec(`
			UPDATE tasks
			SET title = ?, description = ?, acceptance_criteria = ?,
			    upstream_dependency_id = ?, review_required = ?, parent_id = ?,
			    status = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`,
			pt.task.Title, pt.task.Description, pt.task.AcceptanceCriteria,
			upstreamID, pt.task.ReviewRequired, parentID, status, pt.idResult.DBID)

		if err != nil {
			return nil, fmt.Errorf("failed to update task T%d: %w", pt.idResult.DBID, err)
		}
	}

	// Phase 6: Process creates with topological sort
	sortedTasks := topologicalSortParsedTasks(tasksToCreate)

	for _, pt := range sortedTasks {
		status := pt.task.Status
		if status == "" {
			status = "todo"
		}

		// Resolve parent and upstream IDs
		var parentID *int
		var upstreamID *int

		if pt.parentID != nil {
			resolvedID, err := resolveTaskReference(pt.parentID, idMap)
			if err != nil {
				return nil, fmt.Errorf("new task '%s' parent_id: %w", pt.task.Title, err)
			}
			parentID = resolvedID
		}

		if pt.upstreamID != nil {
			resolvedID, err := resolveTaskReference(pt.upstreamID, idMap)
			if err != nil {
				return nil, fmt.Errorf("new task '%s' upstream_dependency_id: %w", pt.task.Title, err)
			}
			upstreamID = resolvedID
		}

		// Insert the new task
		result, err := tx.Exec(`
			INSERT INTO tasks (title, description, acceptance_criteria, upstream_dependency_id, review_required, parent_id, status)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			pt.task.Title, pt.task.Description, pt.task.AcceptanceCriteria,
			upstreamID, pt.task.ReviewRequired, parentID, status)

		if err != nil {
			return nil, fmt.Errorf("failed to insert task '%s': %w", pt.task.Title, err)
		}

		// Get the database-assigned ID
		newID, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("failed to get last insert ID for task '%s': %w", pt.task.Title, err)
		}

		// Add to ID map
		dbID := int(newID)
		if pt.idResult.LocalID != "" {
			idMap[pt.idResult.LocalID] = dbID
		}
		idMap[fmt.Sprintf("T%d", dbID)] = dbID
		idMap[fmt.Sprintf("%d", dbID)] = dbID
	}

	return idMap, nil
}

// resolveTaskReference resolves a task ID reference to a database ID
func resolveTaskReference(idResult *TaskIDResult, idMap map[string]int) (*int, error) {
	if idResult.IsExisting {
		dbID := idResult.DBID
		return &dbID, nil
	}

	if idResult.LocalID != "" {
		if dbID, ok := idMap[idResult.LocalID]; ok {
			return &dbID, nil
		}
		return nil, fmt.Errorf("local reference '%s' not found (task must be defined before it can be referenced)", idResult.LocalID)
	}

	return nil, nil
}

// topologicalSortParsedTasks sorts parsed tasks so dependencies come before dependents
func topologicalSortParsedTasks(tasks []parsedTask) []parsedTask {
	if len(tasks) == 0 {
		return tasks
	}

	// Build a map of local task IDs to their index
	taskMap := make(map[string]int)
	for i, pt := range tasks {
		if pt.idResult.LocalID != "" {
			taskMap[pt.idResult.LocalID] = i
		}
	}

	// Build dependency graph (only for local references, not existing DB tasks)
	dependsOn := make(map[int][]int) // task index -> list of indices that depend on it

	for i, pt := range tasks {
		// Check if parent is a local reference
		if pt.parentID != nil && !pt.parentID.IsExisting && pt.parentID.LocalID != "" {
			if parentIdx, ok := taskMap[pt.parentID.LocalID]; ok {
				dependsOn[parentIdx] = append(dependsOn[parentIdx], i)
			}
		}

		// Check if upstream is a local reference
		if pt.upstreamID != nil && !pt.upstreamID.IsExisting && pt.upstreamID.LocalID != "" {
			if upstreamIdx, ok := taskMap[pt.upstreamID.LocalID]; ok {
				dependsOn[upstreamIdx] = append(dependsOn[upstreamIdx], i)
			}
		}
	}

	// Topological sort using DFS
	visited := make(map[int]bool)
	sorted := make([]parsedTask, 0, len(tasks))

	var visit func(int)
	visit = func(idx int) {
		if visited[idx] {
			return
		}
		visited[idx] = true

		pt := tasks[idx]

		// Visit dependencies first (parent and upstream)
		if pt.parentID != nil && !pt.parentID.IsExisting && pt.parentID.LocalID != "" {
			if parentIdx, ok := taskMap[pt.parentID.LocalID]; ok {
				visit(parentIdx)
			}
		}

		if pt.upstreamID != nil && !pt.upstreamID.IsExisting && pt.upstreamID.LocalID != "" {
			if upstreamIdx, ok := taskMap[pt.upstreamID.LocalID]; ok {
				visit(upstreamIdx)
			}
		}

		sorted = append(sorted, pt)
	}

	// Visit all tasks
	for i := range tasks {
		visit(i)
	}

	return sorted
}

// importTaskLogs imports task log entries
func importTaskLogs(tx *sql.Tx, logs []YAMLTaskLog, idMap map[string]int) error {
	// Get all existing task IDs from database
	existingTaskIDs := make(map[int]bool)
	rows, err := tx.Query("SELECT id FROM tasks")
	if err != nil {
		return fmt.Errorf("failed to query existing task IDs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan task ID: %w", err)
		}
		existingTaskIDs[id] = true
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate existing task IDs: %w", err)
	}

	for i, log := range logs {
		if log.Message == "" {
			return fmt.Errorf("task log %d has empty message", i)
		}

		// Parse task ID
		idResult, err := parseTaskID(log.TaskID, existingTaskIDs)
		if err != nil {
			return fmt.Errorf("task log %d: %w", i, err)
		}

		// Resolve task ID to database ID
		var dbTaskID int
		if idResult.IsExisting {
			dbTaskID = idResult.DBID
		} else if idResult.LocalID != "" {
			var ok bool
			dbTaskID, ok = idMap[idResult.LocalID]
			if !ok {
				return fmt.Errorf("task log %d references unknown local task ID '%s'", i, idResult.LocalID)
			}
		} else {
			return fmt.Errorf("task log %d has invalid task_id", i)
		}

		_, err = tx.Exec(`INSERT INTO task_logs (task_id, message) VALUES (?, ?)`,
			dbTaskID, log.Message)
		if err != nil {
			return fmt.Errorf("failed to insert task log %d: %w", i, err)
		}
	}
	return nil
}

// importTaskReviews imports task review entries
func importTaskReviews(tx *sql.Tx, reviews []YAMLTaskReview, idMap map[string]int) error {
	validStatuses := map[string]bool{
		"pending":  true,
		"approved": true,
		"rejected": true,
	}

	// Get all existing task IDs from database
	existingTaskIDs := make(map[int]bool)
	rows, err := tx.Query("SELECT id FROM tasks")
	if err != nil {
		return fmt.Errorf("failed to query existing task IDs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan task ID: %w", err)
		}
		existingTaskIDs[id] = true
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate existing task IDs: %w", err)
	}

	for i, review := range reviews {
		if review.Message == "" {
			return fmt.Errorf("task review %d has empty message", i)
		}

		status := review.Status
		if status == "" {
			status = "pending"
		}

		if !validStatuses[status] {
			return fmt.Errorf("task review %d has invalid status: %s", i, status)
		}

		// Parse task ID
		idResult, err := parseTaskID(review.TaskID, existingTaskIDs)
		if err != nil {
			return fmt.Errorf("task review %d: %w", i, err)
		}

		// Resolve task ID to database ID
		var dbTaskID int
		if idResult.IsExisting {
			dbTaskID = idResult.DBID
		} else if idResult.LocalID != "" {
			var ok bool
			dbTaskID, ok = idMap[idResult.LocalID]
			if !ok {
				return fmt.Errorf("task review %d references unknown local task ID '%s'", i, idResult.LocalID)
			}
		} else {
			return fmt.Errorf("task review %d has invalid task_id", i)
		}

		_, err = tx.Exec(`
			INSERT INTO task_reviews (task_id, message, attachment, status, feedback)
			VALUES (?, ?, ?, ?, ?)`,
			dbTaskID, review.Message, review.Attachment, status, review.Feedback)
		if err != nil {
			return fmt.Errorf("failed to insert task review %d: %w", i, err)
		}
	}
	return nil
}

// TaskIDResult represents the result of parsing a task ID
type TaskIDResult struct {
	IsExisting bool   // true if this references an existing DB task
	DBID       int    // the database ID (for existing tasks)
	LocalID    string // the local reference ID (for new tasks)
}

// parseTaskID parses a task ID from YAML and determines if it references
// an existing database task or is a local reference for a new task.
// Supported formats:
//   - nil or 0: new task with no local reference
//   - Positive int (e.g., 15): existing database task ID 15 if it exists in existingIDs
//   - String "T15": existing database task ID 15 if it exists in existingIDs
//   - String "new-*" or "tmp-*": new task with local reference
func parseTaskID(idValue interface{}, existingIDs map[int]bool) (*TaskIDResult, error) {
	if idValue == nil {
		return &TaskIDResult{IsExisting: false, LocalID: ""}, nil
	}

	switch v := idValue.(type) {
	case int:
		if v == 0 {
			return &TaskIDResult{IsExisting: false, LocalID: ""}, nil
		}
		if v < 0 {
			return nil, fmt.Errorf("negative task IDs are not allowed: %d", v)
		}
		if existingIDs[v] {
			return &TaskIDResult{IsExisting: true, DBID: v}, nil
		}
		return &TaskIDResult{IsExisting: false, DBID: v}, nil

	case string:
		if v == "" {
			return &TaskIDResult{IsExisting: false, LocalID: ""}, nil
		}

		// Check for T-prefixed database IDs (e.g., "T15")
		if len(v) > 1 && v[0] == 'T' {
			var dbID int
			if _, err := fmt.Sscanf(v, "T%d", &dbID); err == nil && dbID > 0 {
				if existingIDs[dbID] {
					return &TaskIDResult{IsExisting: true, DBID: dbID}, nil
				}
				return &TaskIDResult{IsExisting: false, DBID: dbID}, nil
			}
		}

		// Check for new task local reference IDs (e.g., "new-*", "tmp-*")
		if len(v) >= 4 && (v[:4] == "new-" || v[:4] == "tmp-") {
			return &TaskIDResult{IsExisting: false, LocalID: v}, nil
		}

		return nil, fmt.Errorf("invalid task ID format: %s (use numeric ID, 'T<num>', 'new-*', or 'tmp-*')", v)

	default:
		return nil, fmt.Errorf("invalid task ID type: %T (expected int or string)", idValue)
	}
}

// validateExistingTaskIDs verifies that all referenced database task IDs actually exist
func validateExistingTaskIDs(db *sql.DB, taskIDs []int) error {
	if len(taskIDs) == 0 {
		return nil
	}

	for _, taskID := range taskIDs {
		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM tasks WHERE id = ?)", taskID).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check if task T%d exists: %w", taskID, err)
		}
		if !exists {
			return fmt.Errorf("referenced task T%d does not exist in database", taskID)
		}
	}

	return nil
}
