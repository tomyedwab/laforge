package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Job represents a job in the database
type Job struct {
	ID           int
	JobName      string
	Repository   string
	PRNumber     int
	SHA          string
	Model        string
	PromptType   string
	Status       string
	StartedAt    time.Time
	FinishedAt   sql.NullTime
	LogFile      string
	CreatedAt    time.Time
}

// DB wraps the SQLite database
type DB struct {
	db *sql.DB
}

// New creates a new database connection and initializes the schema
func New(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Create tables
	schema := `
	CREATE TABLE IF NOT EXISTS jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		job_name TEXT UNIQUE NOT NULL,
		repository TEXT NOT NULL,
		pr_number INTEGER NOT NULL,
		sha TEXT,
		model TEXT,
		prompt_type TEXT,
		status TEXT DEFAULT 'running',
		started_at TIMESTAMP,
		finished_at TIMESTAMP,
		log_file TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_jobs_repository ON jobs(repository);
	CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
	CREATE INDEX IF NOT EXISTS idx_jobs_started_at ON jobs(started_at DESC);
	`

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return &DB{db: db}, nil
}

// Close closes the database connection
func (d *DB) Close() error {
	return d.db.Close()
}

// InsertJob inserts or updates a job in the database
func (d *DB) InsertJob(job *Job) error {
	query := `
	INSERT INTO jobs (job_name, repository, pr_number, sha, model, prompt_type, status, started_at, finished_at, log_file)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(job_name) DO UPDATE SET
		status = excluded.status,
		finished_at = excluded.finished_at
	`

	_, err := d.db.Exec(query,
		job.JobName,
		job.Repository,
		job.PRNumber,
		job.SHA,
		job.Model,
		job.PromptType,
		job.Status,
		job.StartedAt,
		job.FinishedAt,
		job.LogFile,
	)

	return err
}

// UpdateJobStatus updates the status and finished_at time for a job
func (d *DB) UpdateJobStatus(jobName, status string, finishedAt time.Time) error {
	query := `UPDATE jobs SET status = ?, finished_at = ? WHERE job_name = ?`
	_, err := d.db.Exec(query, status, finishedAt, jobName)
	return err
}

// GetAllJobs retrieves all jobs ordered by started_at descending
func (d *DB) GetAllJobs() ([]Job, error) {
	query := `
	SELECT id, job_name, repository, pr_number, sha, model, prompt_type, status, started_at, finished_at, log_file, created_at
	FROM jobs
	ORDER BY started_at DESC
	`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		err := rows.Scan(
			&job.ID,
			&job.JobName,
			&job.Repository,
			&job.PRNumber,
			&job.SHA,
			&job.Model,
			&job.PromptType,
			&job.Status,
			&job.StartedAt,
			&job.FinishedAt,
			&job.LogFile,
			&job.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// GetJobByName retrieves a specific job by name
func (d *DB) GetJobByName(jobName string) (*Job, error) {
	query := `
	SELECT id, job_name, repository, pr_number, sha, model, prompt_type, status, started_at, finished_at, log_file, created_at
	FROM jobs
	WHERE job_name = ?
	`

	var job Job
	err := d.db.QueryRow(query, jobName).Scan(
		&job.ID,
		&job.JobName,
		&job.Repository,
		&job.PRNumber,
		&job.SHA,
		&job.Model,
		&job.PromptType,
		&job.Status,
		&job.StartedAt,
		&job.FinishedAt,
		&job.LogFile,
		&job.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &job, nil
}
