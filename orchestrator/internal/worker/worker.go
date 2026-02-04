package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/tom/laforge/orchestrator/internal/docker"
	"github.com/tom/laforge/orchestrator/internal/gitea"
	"github.com/tom/laforge/orchestrator/internal/notify"
	"github.com/tom/laforge/orchestrator/internal/queue"
)

// Server wraps the Asynq server for processing tasks
type Server struct {
	asynq       *asynq.Server
	mux         *asynq.ServeMux
	gitea       *gitea.Client
	notify      *notify.Client
	redisAddr   string
	giteaURL    string
	giteaToken  string
	gitImage    string
	botUsername string
	botEmail    string
}

// Config holds the configuration for the worker server
type Config struct {
	RedisAddr    string
	Concurrency  int
	GiteaClient  *gitea.Client
	NotifyClient *notify.Client
	GiteaURL     string
	GiteaToken   string
	GitImage     string
	BotUsername  string
	BotEmail     string
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
		asynq:       srv,
		mux:         mux,
		gitea:       cfg.GiteaClient,
		notify:      cfg.NotifyClient,
		redisAddr:   cfg.RedisAddr,
		giteaURL:    cfg.GiteaURL,
		giteaToken:  cfg.GiteaToken,
		gitImage:    cfg.GitImage,
		botUsername: cfg.BotUsername,
		botEmail:    cfg.BotEmail,
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
	if err := s.gitea.UpdateStatus(ctx, payload.HeadRepository, payload.SHA, gitea.StatusRunning); err != nil {
		slog.Error("failed to update status to running", "error", err)
		// Continue processing even if status update fails
	}

	// Process the job
	if err := s.processJob(ctx, &payload); err != nil {
		slog.Error("job failed",
			"repository", payload.Repository,
			"pr_number", payload.PRNumber,
			"error", err,
		)

		// Update Gitea status to "failure"
		if err := s.gitea.UpdateStatus(ctx, payload.HeadRepository, payload.SHA, gitea.StatusFailure); err != nil {
			slog.Error("failed to update status to failure", "error", err)
		}

		// Send failure notification only if retries are exhausted
		// Asynq will retry the task, so we check if this is the final attempt
		retried, ok := asynq.GetRetryCount(ctx)
		if !ok {
			slog.Warn("failed to get retry count", "error", err)
		}
		maxRetry, ok := asynq.GetMaxRetry(ctx)
		if !ok {
			slog.Warn("failed to get max retry", "error", err)
		}

		// MaxRetry is 3, so after 3 retries (Retried=3), this is the final failure
		if retried >= maxRetry {
			slog.Info("retries exhausted, sending failure notification",
				"repository", payload.Repository,
				"pr_number", payload.PRNumber,
				"retried", retried,
				"max_retry", maxRetry,
			)
			if err := s.notify.NotifyFailure(ctx, payload.Repository, payload.PRNumber); err != nil {
				slog.Error("failed to send failure notification", "error", err)
				// Don't fail the job if notification fails
			}
		}

		return fmt.Errorf("job processing failed: %w", err)
	}

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

	// Send success notification
	if err := s.notify.NotifySuccess(ctx, payload.Repository, payload.PRNumber); err != nil {
		slog.Error("failed to send success notification", "error", err)
		// Don't fail the job if notification fails
	}

	return nil
}

// processJob handles the actual PR job processing
func (s *Server) processJob(ctx context.Context, payload *queue.PRJobPayload) error {
	// Generate unique job name
	jobName := fmt.Sprintf("laforge-pr-%s-%d-%d",
		strings.ReplaceAll(payload.Repository, "/", "-"),
		payload.PRNumber,
		time.Now().Unix(),
	)

	slog.Info("setting up workspace",
		"volume", jobName,
		"repository", payload.Repository,
		"pr_number", payload.PRNumber,
	)

	// Initialize Docker client
	dockerClient, err := docker.NewClient(jobName)
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	// Pull git image if needed
	slog.Info("pulling git image", "image", s.gitImage)
	if err := dockerClient.PullImage(ctx, s.gitImage); err != nil {
		return fmt.Errorf("failed to pull git image: %w", err)
	}

	// Create Docker volume
	if err := dockerClient.CreateVolume(ctx, jobName); err != nil {
		return fmt.Errorf("failed to create volume: %w", err)
	}

	// Ensure volume cleanup on exit
	defer func() {
		cleanupCtx := context.Background()
		if err := dockerClient.DeleteVolume(cleanupCtx, jobName); err != nil {
			slog.Error("failed to delete volume", "volume", jobName, "error", err)
		}
	}()

	// Construct git clone URL with authentication
	cloneURL, err := gitea.GetCloneURL(s.giteaURL, s.giteaToken, payload.Repository, payload.SHA)
	if err != nil {
		return fmt.Errorf("failed to construct clone URL: %w", err)
	}

	// Run init container to clone repository
	if err := dockerClient.RunInitContainer(ctx, jobName, cloneURL, payload.SHA, s.gitImage); err != nil {
		return fmt.Errorf("failed to run init container: %w", err)
	}

	// Fetch PR history and attachments
	fetcher := gitea.NewFetcher(s.gitea.GetSDKClient(), s.giteaURL, s.giteaToken)
	history, err := fetcher.FetchPRHistory(ctx, payload.Repository, payload.PRNumber)
	if err != nil {
		return fmt.Errorf("failed to fetch PR history: %w", err)
	}

	// Prepare files to copy to volume
	files := make(map[string][]byte)
	files[".pr/history.md"] = []byte(history.Markdown)

	// Add attachment files
	for _, att := range history.Attachments {
		if len(att.Content) > 0 {
			files[att.LocalPath] = att.Content
		}
	}

	// Copy .pr files to volume
	if err := dockerClient.CopyFilesToVolume(ctx, jobName, files, s.gitImage); err != nil {
		return fmt.Errorf("failed to copy files to volume: %w", err)
	}

	/* TODO: If we deploy images to a repository, may need to pull the latest version
	// Pull agent image if needed
	slog.Info("pulling agent image", "image", payload.ModelImage)
	if err := dockerClient.PullImage(ctx, payload.ModelImage); err != nil {
		return fmt.Errorf("failed to pull agent image: %w", err)
	}
	*/

	// Run agent container
	slog.Info("running agent container", "volume", jobName, "model_image", payload.ModelImage)

	if err := dockerClient.RunAgentContainer(ctx, jobName, payload.ModelImage, payload.PromptType, payload.Model); err != nil {
		return fmt.Errorf("failed to run agent container: %w", err)
	}

	// Agent completed successfully - now run teardown to collect and commit changes
	slog.Info("agent completed successfully, running teardown")

	// Extract files we need from the volume
	filesToExtract := []string{".pr/commit.md", ".pr/status.yaml"}
	extractedFiles, err := dockerClient.CopyFilesFromVolume(ctx, jobName, filesToExtract, s.gitImage)
	if err != nil {
		return fmt.Errorf("failed to extract files from volume: %w", err)
	}

	// Commit and push changes to the PR branch
	commitMessage := extractedFiles[".pr/commit.md"]
	headSHA, err := dockerClient.CommitAndPushChanges(
		ctx,
		jobName,
		s.gitImage,
		s.giteaURL,
		s.giteaToken,
		payload.Repository,
		payload.Branch,
		s.botUsername,
		s.botEmail,
		commitMessage,
	)
	if err != nil {
		return fmt.Errorf("failed to commit and push changes: %w", err)
	}

	// If there were changes committed, use the new HEAD SHA for status posting
	// Otherwise, use the original SHA
	statusCommitSHA := payload.SHA
	if headSHA != "" {
		statusCommitSHA = headSHA
		slog.Info("changes committed to PR", "new_head_sha", headSHA)
	} else {
		slog.Info("no changes to commit")
	}

	// Post status updates to the PR if status.yaml exists
	if statusYAML, ok := extractedFiles[".pr/status.yaml"]; ok {
		if err := gitea.PostStatus(ctx, statusYAML, statusCommitSHA, payload.Repository, payload.PRNumber, s.gitea); err != nil {
			return fmt.Errorf("failed to post status to PR: %w", err)
		}
	} else {
		slog.Info("no status.yaml found, skipping status post")
	}

	slog.Info("teardown completed successfully")
	return nil
}
