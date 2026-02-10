package watcher

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/tom/laforge/viewer/internal/db"
)

// Watcher watches for new log files and indexes them
type Watcher struct {
	logsDir string
	db      *db.DB
	watcher *fsnotify.Watcher
}

// New creates a new file watcher
func New(logsDir string, database *db.DB) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	// Add the logs directory to the watcher
	if err := watcher.Add(logsDir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch directory %s: %w", logsDir, err)
	}

	w := &Watcher{
		logsDir: logsDir,
		db:      database,
		watcher: watcher,
	}

	// Index existing files on startup
	if err := w.indexExistingFiles(); err != nil {
		slog.Error("failed to index existing files", "error", err)
		// Continue even if indexing fails
	}

	return w, nil
}

// indexExistingFiles indexes all existing .jsonl files in the logs directory
func (w *Watcher) indexExistingFiles() error {
	slog.Info("indexing existing log files", "dir", w.logsDir)

	entries, err := os.ReadDir(w.logsDir)
	if err != nil {
		return fmt.Errorf("failed to read logs directory: %w", err)
	}

	indexed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		logFile := filepath.Join(w.logsDir, entry.Name())
		if err := w.indexFile(logFile); err != nil {
			slog.Error("failed to index file", "file", logFile, "error", err)
			// Continue to next file
			continue
		}
		indexed++
	}

	slog.Info("indexed existing log files", "count", indexed)
	return nil
}

// indexFile reads the metadata from a log file and stores it in the database
func (w *Watcher) indexFile(logFile string) error {
	// Check if already indexed
	jobName := strings.TrimSuffix(filepath.Base(logFile), ".jsonl")
	existing, err := w.db.GetJobByName(jobName)
	if err != nil {
		return fmt.Errorf("failed to check if job exists: %w", err)
	}

	// Read the first line to get metadata
	file, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Increase buffer size to handle large JSONL lines (e.g., tool results with file contents)
	scanner.Buffer(make([]byte, 0), 10*1024*1024) // 10MB max
	var job *db.Job

	// Process lines to find job_start and job_end
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Skip invalid JSON lines
			continue
		}

		entryType, ok := entry["type"].(string)
		if !ok {
			continue
		}

		if entryType == "job_start" {
			// Parse job metadata
			job = &db.Job{
				JobName:    getString(entry, "job_name"),
				Repository: getString(entry, "repository"),
				PRNumber:   int(getFloat(entry, "pr_number")),
				SHA:        getString(entry, "sha"),
				Model:      getString(entry, "model"),
				PromptType: getString(entry, "prompt_type"),
				Status:     "running",
				LogFile:    logFile,
			}

			// Parse started_at timestamp
			if startedAtStr := getString(entry, "started_at"); startedAtStr != "" {
				startedAt, err := time.Parse(time.RFC3339, startedAtStr)
				if err == nil {
					job.StartedAt = startedAt
				}
			}
		} else if entryType == "job_end" && job != nil {
			// Update job status
			job.Status = getString(entry, "status")

			// Parse finished_at timestamp
			if finishedAtStr := getString(entry, "finished_at"); finishedAtStr != "" {
				finishedAt, err := time.Parse(time.RFC3339, finishedAtStr)
				if err == nil {
					if existing != nil {
						// Update existing job
						if err := w.db.UpdateJobStatus(job.JobName, job.Status, finishedAt); err != nil {
							return fmt.Errorf("failed to update job status: %w", err)
						}
						return nil
					} else {
						// Set finished_at for new job
						job.FinishedAt = sql.NullTime{Time: finishedAt, Valid: true}
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading log file: %w", err)
	}

	// Insert job if we found metadata
	if job != nil && existing == nil {
		if err := w.db.InsertJob(job); err != nil {
			return fmt.Errorf("failed to insert job: %w", err)
		}
		slog.Info("indexed new job", "job_name", job.JobName, "repository", job.Repository, "pr", job.PRNumber)
	}

	return nil
}

// getString safely extracts a string from a map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// getFloat safely extracts a float64 from a map
func getFloat(m map[string]interface{}, key string) float64 {
	if val, ok := m[key].(float64); ok {
		return val
	}
	return 0
}

// Start starts the file watcher
func (w *Watcher) Start() {
	slog.Info("starting file watcher", "dir", w.logsDir)

	go func() {
		for {
			select {
			case event, ok := <-w.watcher.Events:
				if !ok {
					return
				}

				// Only process create and write events for .jsonl files
				if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
					if strings.HasSuffix(event.Name, ".jsonl") {
						slog.Info("detected log file change", "file", event.Name, "op", event.Op.String())
						// Small delay to ensure file is fully written
						time.Sleep(100 * time.Millisecond)
						if err := w.indexFile(event.Name); err != nil {
							slog.Error("failed to index file", "file", event.Name, "error", err)
						}
					}
				}

			case err, ok := <-w.watcher.Errors:
				if !ok {
					return
				}
				slog.Error("watcher error", "error", err)
			}
		}
	}()
}

// Close stops the file watcher
func (w *Watcher) Close() error {
	return w.watcher.Close()
}
