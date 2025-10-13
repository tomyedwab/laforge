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

// YAMLTask represents a task in the YAML import format
type YAMLTask struct {
	ID                   int    `yaml:"id"`
	Title                string `yaml:"title"`
	Description          string `yaml:"description"`
	AcceptanceCriteria   string `yaml:"acceptance_criteria"`
	UpstreamDependencyID *int   `yaml:"upstream_dependency_id"`
	ReviewRequired       bool   `yaml:"review_required"`
	ParentID             *int   `yaml:"parent_id"`
	Status               string `yaml:"status"`
}

// YAMLTaskLog represents a task log entry in the YAML import format
type YAMLTaskLog struct {
	TaskID  int    `yaml:"task_id"`
	Message string `yaml:"message"`
}

// YAMLTaskReview represents a task review in the YAML import format
type YAMLTaskReview struct {
	TaskID     int     `yaml:"task_id"`
	Message    string  `yaml:"message"`
	Attachment *string `yaml:"attachment"`
	Status     string  `yaml:"status"`
	Feedback   *string `yaml:"feedback"`
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

// importTasks validates and imports tasks, handling dependencies correctly
// Returns a map from YAML task IDs to database-assigned task IDs
func importTasks(tx *sql.Tx, tasks []YAMLTask) (map[int]int, error) {
	if len(tasks) == 0 {
		return make(map[int]int), nil
	}

	// Validate task statuses
	validStatuses := map[string]bool{
		"todo":        true,
		"in-progress": true,
		"in-review":   true,
		"completed":   true,
	}

	for _, task := range tasks {
		if task.Title == "" {
			return nil, fmt.Errorf("task with ID %d has empty title", task.ID)
		}

		// Set default status if not provided
		status := task.Status
		if status == "" {
			status = "todo"
		}

		if !validStatuses[status] {
			return nil, fmt.Errorf("task %d has invalid status: %s", task.ID, status)
		}
	}

	// Build a map of task IDs for reference validation
	taskIDMap := make(map[int]bool)
	for _, task := range tasks {
		if task.ID > 0 {
			taskIDMap[task.ID] = true
		}
	}

	// Validate references
	for _, task := range tasks {
		if task.ParentID != nil {
			if !taskIDMap[*task.ParentID] {
				return nil, fmt.Errorf("task %d references non-existent parent task %d", task.ID, *task.ParentID)
			}
		}
		if task.UpstreamDependencyID != nil {
			if !taskIDMap[*task.UpstreamDependencyID] {
				return nil, fmt.Errorf("task %d references non-existent upstream dependency %d", task.ID, *task.UpstreamDependencyID)
			}
		}
	}

	// Sort tasks to ensure parent tasks and upstream dependencies are inserted first
	sortedTasks := topologicalSortTasks(tasks)

	// Map from YAML ID to database-assigned ID
	yamlIDToDBID := make(map[int]int)

	// Insert tasks in order
	for _, task := range sortedTasks {
		status := task.Status
		if status == "" {
			status = "todo"
		}

		// Resolve references using the ID map
		var parentID *int
		var upstreamDependencyID *int

		if task.ParentID != nil {
			if dbID, ok := yamlIDToDBID[*task.ParentID]; ok {
				parentID = &dbID
			} else {
				return nil, fmt.Errorf("task '%s' references parent ID %d which hasn't been inserted yet", task.Title, *task.ParentID)
			}
		}

		if task.UpstreamDependencyID != nil {
			if dbID, ok := yamlIDToDBID[*task.UpstreamDependencyID]; ok {
				upstreamDependencyID = &dbID
			} else {
				return nil, fmt.Errorf("task '%s' references upstream dependency ID %d which hasn't been inserted yet", task.Title, *task.UpstreamDependencyID)
			}
		}

		// Always let database auto-generate IDs
		result, err := tx.Exec(`
			INSERT INTO tasks (title, description, acceptance_criteria, upstream_dependency_id, review_required, parent_id, status)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			task.Title, task.Description, task.AcceptanceCriteria,
			upstreamDependencyID, task.ReviewRequired, parentID, status)

		if err != nil {
			return nil, fmt.Errorf("failed to insert task '%s': %w", task.Title, err)
		}

		// Get the database-assigned ID
		newID, err := result.LastInsertId()
		if err != nil {
			return nil, fmt.Errorf("failed to get last insert ID for task '%s': %w", task.Title, err)
		}

		// Map YAML ID to database ID
		if task.ID > 0 {
			yamlIDToDBID[task.ID] = int(newID)
		}
	}

	return yamlIDToDBID, nil
}

// topologicalSortTasks sorts tasks so that parent tasks and upstream dependencies come before dependent tasks
func topologicalSortTasks(tasks []YAMLTask) []YAMLTask {
	// Build adjacency list for dependencies
	taskMap := make(map[int]YAMLTask)
	dependsOn := make(map[int][]int) // task ID -> list of tasks that depend on it

	for _, task := range tasks {
		if task.ID > 0 {
			taskMap[task.ID] = task
		}
	}

	// Build dependency graph
	for _, task := range tasks {
		if task.ID > 0 {
			if task.ParentID != nil {
				dependsOn[*task.ParentID] = append(dependsOn[*task.ParentID], task.ID)
			}
			if task.UpstreamDependencyID != nil {
				dependsOn[*task.UpstreamDependencyID] = append(dependsOn[*task.UpstreamDependencyID], task.ID)
			}
		}
	}

	// Topological sort using DFS
	visited := make(map[int]bool)
	sorted := make([]YAMLTask, 0, len(tasks))

	var visit func(int)
	visit = func(taskID int) {
		if visited[taskID] {
			return
		}
		visited[taskID] = true

		// Visit dependencies first
		task := taskMap[taskID]
		if task.ParentID != nil {
			visit(*task.ParentID)
		}
		if task.UpstreamDependencyID != nil {
			visit(*task.UpstreamDependencyID)
		}

		sorted = append(sorted, task)
	}

	// Visit all tasks
	for _, task := range tasks {
		if task.ID > 0 {
			visit(task.ID)
		} else {
			// Tasks without IDs go at the end
			sorted = append(sorted, task)
		}
	}

	return sorted
}

// importTaskLogs imports task log entries
func importTaskLogs(tx *sql.Tx, logs []YAMLTaskLog, yamlIDToDBID map[int]int) error {
	for _, log := range logs {
		if log.TaskID == 0 {
			return fmt.Errorf("task log has invalid task_id: %d", log.TaskID)
		}
		if log.Message == "" {
			return fmt.Errorf("task log for task %d has empty message", log.TaskID)
		}

		// Map YAML task ID to database task ID
		dbTaskID, ok := yamlIDToDBID[log.TaskID]
		if !ok {
			return fmt.Errorf("task log references non-existent task ID %d", log.TaskID)
		}

		_, err := tx.Exec(`INSERT INTO task_logs (task_id, message) VALUES (?, ?)`,
			dbTaskID, log.Message)
		if err != nil {
			return fmt.Errorf("failed to insert task log for task %d: %w", log.TaskID, err)
		}
	}
	return nil
}

// importTaskReviews imports task review entries
func importTaskReviews(tx *sql.Tx, reviews []YAMLTaskReview, yamlIDToDBID map[int]int) error {
	validStatuses := map[string]bool{
		"pending":  true,
		"approved": true,
		"rejected": true,
	}

	for _, review := range reviews {
		if review.TaskID == 0 {
			return fmt.Errorf("task review has invalid task_id: %d", review.TaskID)
		}
		if review.Message == "" {
			return fmt.Errorf("task review for task %d has empty message", review.TaskID)
		}

		status := review.Status
		if status == "" {
			status = "pending"
		}

		if !validStatuses[status] {
			return fmt.Errorf("task review for task %d has invalid status: %s", review.TaskID, status)
		}

		// Map YAML task ID to database task ID
		dbTaskID, ok := yamlIDToDBID[review.TaskID]
		if !ok {
			return fmt.Errorf("task review references non-existent task ID %d", review.TaskID)
		}

		_, err := tx.Exec(`
			INSERT INTO task_reviews (task_id, message, attachment, status, feedback)
			VALUES (?, ?, ?, ?, ?)`,
			dbTaskID, review.Message, review.Attachment, status, review.Feedback)
		if err != nil {
			return fmt.Errorf("failed to insert task review for task %d: %w", review.TaskID, err)
		}
	}
	return nil
}
