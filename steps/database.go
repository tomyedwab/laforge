package steps

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// StepDatabase provides database operations for steps
type StepDatabase struct {
	db *sql.DB
}

// InitStepDB initializes a new step database at the specified path
func InitStepDB(dbPath string) (*StepDatabase, error) {
	// Ensure directory exists
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open step database: %w", err)
	}

	// Enable foreign key constraints
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Create schema
	if err := createStepSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create step schema: %w", err)
	}

	return &StepDatabase{db: db}, nil
}

// createStepSchema creates the step database schema
func createStepSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS steps (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		active BOOLEAN DEFAULT TRUE,
		parent_step_id INTEGER,
		commit_sha_before TEXT NOT NULL,
		commit_sha_after TEXT,
		agent_config_json TEXT NOT NULL,
		start_time TIMESTAMP NOT NULL,
		end_time TIMESTAMP,
		duration_ms INTEGER,
		token_usage_json TEXT NOT NULL DEFAULT '{}',
		exit_code INTEGER,
		project_id TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (parent_step_id) REFERENCES steps(id)
	);

	CREATE INDEX IF NOT EXISTS idx_steps_project_id ON steps(project_id);
	CREATE INDEX IF NOT EXISTS idx_steps_active ON steps(active);
	CREATE INDEX IF NOT EXISTS idx_steps_parent_step_id ON steps(parent_step_id);
	CREATE INDEX IF NOT EXISTS idx_steps_created_at ON steps(created_at);`

	_, err := db.Exec(schema)
	return err
}

// CreateStep creates a new step record
func (sdb *StepDatabase) CreateStep(step *Step) error {
	if step == nil {
		return fmt.Errorf("step cannot be nil")
	}

	if step.ProjectID == "" {
		return fmt.Errorf("project_id is required")
	}

	if step.CommitSHABefore == "" {
		return fmt.Errorf("commit_sha_before is required")
	}

	stepJSON, err := step.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize step: %w", err)
	}

	result, err := sdb.db.Exec(`
		INSERT INTO steps (
			active, parent_step_id, commit_sha_before, commit_sha_after,
			agent_config_json, start_time, end_time, duration_ms,
			token_usage_json, exit_code, project_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		stepJSON.Active, stepJSON.ParentStepID, stepJSON.CommitSHABefore, stepJSON.CommitSHAAfter,
		stepJSON.AgentConfigJSON, stepJSON.StartTime, stepJSON.EndTime, stepJSON.DurationMs,
		stepJSON.TokenUsageJSON, stepJSON.ExitCode, stepJSON.ProjectID)

	if err != nil {
		return fmt.Errorf("failed to insert step: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	step.ID = int(id)
	return nil
}

// GetStep retrieves a step by ID
func (sdb *StepDatabase) GetStep(stepID int) (*Step, error) {
	var stepJSON StepJSON
	err := sdb.db.QueryRow(`
		SELECT id, active, parent_step_id, commit_sha_before, commit_sha_after,
		       agent_config_json, start_time, end_time, duration_ms,
		       token_usage_json, exit_code, project_id, created_at
		FROM steps
		WHERE id = ?`, stepID).Scan(
		&stepJSON.ID, &stepJSON.Active, &stepJSON.ParentStepID, &stepJSON.CommitSHABefore,
		&stepJSON.CommitSHAAfter, &stepJSON.AgentConfigJSON, &stepJSON.StartTime,
		&stepJSON.EndTime, &stepJSON.DurationMs, &stepJSON.TokenUsageJSON,
		&stepJSON.ExitCode, &stepJSON.ProjectID, &stepJSON.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query step: %w", err)
	}

	return stepJSON.FromJSON()
}

// GetLatestActiveStep returns the most recent active step for a project
func (sdb *StepDatabase) GetLatestActiveStep(projectID string) (*Step, error) {
	var stepJSON StepJSON
	err := sdb.db.QueryRow(`
		SELECT id, active, parent_step_id, commit_sha_before, commit_sha_after,
		       agent_config_json, start_time, end_time, duration_ms,
		       token_usage_json, exit_code, project_id, created_at
		FROM steps
		WHERE project_id = ? AND active = TRUE
		ORDER BY id DESC
		LIMIT 1`, projectID).Scan(
		&stepJSON.ID, &stepJSON.Active, &stepJSON.ParentStepID, &stepJSON.CommitSHABefore,
		&stepJSON.CommitSHAAfter, &stepJSON.AgentConfigJSON, &stepJSON.StartTime,
		&stepJSON.EndTime, &stepJSON.DurationMs, &stepJSON.TokenUsageJSON,
		&stepJSON.ExitCode, &stepJSON.ProjectID, &stepJSON.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query latest active step: %w", err)
	}

	return stepJSON.FromJSON()
}

// UpdateStep updates a step with completion data
func (sdb *StepDatabase) UpdateStep(stepID int, commitSHAAfter string, endTime time.Time, durationMs int, exitCode int, tokenUsage TokenUsage) error {
	tokenUsageJSON, err := json.Marshal(tokenUsage)
	if err != nil {
		return fmt.Errorf("failed to serialize token usage: %w", err)
	}

	_, err = sdb.db.Exec(`
		UPDATE steps
		SET commit_sha_after = ?, end_time = ?, duration_ms = ?, exit_code = ?, token_usage_json = ?
		WHERE id = ?`, commitSHAAfter, endTime, durationMs, exitCode, string(tokenUsageJSON), stepID)

	if err != nil {
		return fmt.Errorf("failed to update step: %w", err)
	}

	return nil
}

// DeactivateStep marks a step as inactive (for rollback functionality)
func (sdb *StepDatabase) DeactivateStep(stepID int) error {
	_, err := sdb.db.Exec(`UPDATE steps SET active = FALSE WHERE id = ?`, stepID)
	if err != nil {
		return fmt.Errorf("failed to deactivate step: %w", err)
	}
	return nil
}

// DeactivateStepsFromID deactivates all steps with ID >= given stepID (for rollback functionality)
func (sdb *StepDatabase) DeactivateStepsFromID(stepID int) error {
	_, err := sdb.db.Exec(`UPDATE steps SET active = FALSE WHERE id >= ?`, stepID)
	if err != nil {
		return fmt.Errorf("failed to deactivate steps from ID %d: %w", stepID, err)
	}
	return nil
}

// ListSteps returns all steps for a project, optionally filtered by active status
func (sdb *StepDatabase) ListSteps(projectID string, activeOnly bool) ([]*Step, error) {
	query := `
		SELECT id, active, parent_step_id, commit_sha_before, commit_sha_after,
		       agent_config_json, start_time, end_time, duration_ms,
		       token_usage_json, exit_code, project_id, created_at
		FROM steps
		WHERE project_id = ?`

	if activeOnly {
		query += " AND active = TRUE"
	}

	query += " ORDER BY id DESC"

	rows, err := sdb.db.Query(query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query steps: %w", err)
	}
	defer rows.Close()

	var steps []*Step
	for rows.Next() {
		var stepJSON StepJSON
		if err := rows.Scan(
			&stepJSON.ID, &stepJSON.Active, &stepJSON.ParentStepID, &stepJSON.CommitSHABefore,
			&stepJSON.CommitSHAAfter, &stepJSON.AgentConfigJSON, &stepJSON.StartTime,
			&stepJSON.EndTime, &stepJSON.DurationMs, &stepJSON.TokenUsageJSON,
			&stepJSON.ExitCode, &stepJSON.ProjectID, &stepJSON.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan step: %w", err)
		}

		step, err := stepJSON.FromJSON()
		if err != nil {
			return nil, fmt.Errorf("failed to deserialize step: %w", err)
		}

		steps = append(steps, step)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate steps: %w", err)
	}

	return steps, nil
}

// GetStepCount returns the total number of steps for a project
func (sdb *StepDatabase) GetStepCount(projectID string) (int, error) {
	var count int
	err := sdb.db.QueryRow(`SELECT COUNT(*) FROM steps WHERE project_id = ?`, projectID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count steps: %w", err)
	}
	return count, nil
}

// GetNextStepID returns the next step ID that would be assigned
func (sdb *StepDatabase) GetNextStepID() (int, error) {
	var maxID sql.NullInt64
	err := sdb.db.QueryRow(`SELECT MAX(id) FROM steps`).Scan(&maxID)
	if err != nil {
		return 1, fmt.Errorf("failed to get max step ID: %w", err)
	}

	if maxID.Valid {
		return int(maxID.Int64) + 1, nil
	}
	return 1, nil
}

// Close closes the database connection
func (sdb *StepDatabase) Close() error {
	if sdb.db != nil {
		return sdb.db.Close()
	}
	return nil
}

// GetDB returns the underlying database connection (for testing)
func (sdb *StepDatabase) GetDB() *sql.DB {
	return sdb.db
}
