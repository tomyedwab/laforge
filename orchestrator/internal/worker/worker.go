package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/tom/laforge/orchestrator/internal/gitea"
	"github.com/tom/laforge/orchestrator/internal/queue"
)

// Server wraps the Asynq server for processing tasks
type Server struct {
	asynq     *asynq.Server
	mux       *asynq.ServeMux
	gitea     *gitea.Client
	redisAddr string
}

// Config holds the configuration for the worker server
type Config struct {
	RedisAddr   string
	Concurrency int
	GiteaClient *gitea.Client
}

// NewServer creates a new worker server
func NewServer(cfg Config) *Server {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr},
		asynq.Config{
			Concurrency: cfg.Concurrency,
			Queues: map[string]int{
				queue.QueueDefault: 10, // Priority 10 for default queue
			},
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				// Exponential backoff: 30s, 1m, 2m
				return time.Duration(1<<uint(n)) * 30 * time.Second
			},
		},
	)

	mux := asynq.NewServeMux()

	return &Server{
		asynq:     srv,
		mux:       mux,
		gitea:     cfg.GiteaClient,
		redisAddr: cfg.RedisAddr,
	}
}

// RegisterHandlers registers all task handlers
func (s *Server) RegisterHandlers() {
	s.mux.HandleFunc(queue.TaskTypePRJob, s.handlePRJob)
}

// Start starts the worker server
func (s *Server) Start() error {
	slog.Info("starting worker server")
	return s.asynq.Run(s.mux)
}

// Shutdown gracefully shuts down the worker server
func (s *Server) Shutdown() {
	slog.Info("shutting down worker server")
	s.asynq.Shutdown()
}

// handlePRJob processes a PR job
func (s *Server) handlePRJob(ctx context.Context, t *asynq.Task) error {
	// Parse the payload
	var payload queue.PRJobPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	slog.Info("processing PR job",
		"repository", payload.Repository,
		"head_repository", payload.HeadRepository,
		"pr_number", payload.PRNumber,
		"sha", payload.SHA,
		"action", payload.Action,
	)

	// Acquire lock for this PR
	lock := queue.NewLock(s.redisAddr, payload.Repository, payload.PRNumber)
	defer lock.Close()

	acquired, err := lock.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !acquired {
		// Another worker is processing this PR, drop the task silently
		slog.Info("lock already held for PR, dropping task",
			"repository", payload.Repository,
			"pr_number", payload.PRNumber,
		)
		return nil
	}
	defer lock.Release(ctx)

	// Update Gitea status to "running"
	if err := s.gitea.UpdateStatus(ctx, payload.Repository, payload.SHA, gitea.StatusRunning); err != nil {
		slog.Error("failed to update status to running", "error", err)
		// Continue processing even if status update fails
	}

	// Process the job (for now, just sleep for 1 minute)
	slog.Info("job running, sleeping for 1 minute",
		"repository", payload.Repository,
		"pr_number", payload.PRNumber,
	)

	select {
	case <-time.After(1 * time.Minute):
		// Job completed successfully
		slog.Info("job completed",
			"repository", payload.Repository,
			"pr_number", payload.PRNumber,
		)

		// Update Gitea status to "success"
		if err := s.gitea.UpdateStatus(ctx, payload.Repository, payload.SHA, gitea.StatusSuccess); err != nil {
			slog.Error("failed to update status to success", "error", err)
			return fmt.Errorf("failed to update final status: %w", err)
		}

		return nil

	case <-ctx.Done():
		// Context cancelled
		slog.Warn("job cancelled",
			"repository", payload.Repository,
			"pr_number", payload.PRNumber,
		)

		// Update Gitea status to "failure"
		if err := s.gitea.UpdateStatus(ctx, payload.Repository, payload.SHA, gitea.StatusFailure); err != nil {
			slog.Error("failed to update status to failure", "error", err)
		}

		return ctx.Err()
	}
}
