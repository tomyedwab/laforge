package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

const (
	// Task types
	TaskTypePRJob = "pr:job"

	// Queue names
	QueueDefault = "default"

	// Redis lock configuration
	// Lock TTL needs to be long enough for AI agent runs which can take 30+ minutes
	lockTTL = 60 * time.Minute
)

// PRJobPayload represents the payload for a PR job
type PRJobPayload struct {
	Repository      string `json:"repository"`        // Full repository name (e.g., "owner/repo")
	PRNumber        int    `json:"pr_number"`         // Pull request number
	SHA             string `json:"sha"`               // Commit SHA
	Action          string `json:"action"`            // Webhook action (e.g., "opened", "synchronized")
	Sender          string `json:"sender"`            // User who triggered the webhook
	HeadRepository  string `json:"head_repository"`   // Head repository (may differ for forks)
	Branch          string `json:"branch"`            // PR branch name (head ref)
	PromptType      string `json:"prompt_type"`       // Prompt type (e.g., "implement", "plan", "critique", "cleanup")
	Model           string `json:"model"`             // Full model ID (e.g., "claude-sonnet-4-5-20250929")
	ModelImage      string `json:"model_image"`       // Container image for the model (e.g., "laforge/claudecode:sonnet")
	IsCleanupAction bool   `json:"is_cleanup_action"` // True if this is a cleanup action (bypasses agent processing)
}

// Client wraps the Asynq client for enqueueing tasks
type Client struct {
	asynq *asynq.Client
}

// NewClient creates a new queue client
func NewClient(redisAddr string) *Client {
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	return &Client{asynq: client}
}

// EnqueuePRJob enqueues a PR job with uniqueness constraint
func (c *Client) EnqueuePRJob(ctx context.Context, payload PRJobPayload) error {
	// Serialize the payload
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create the task with uniqueness key
	task := asynq.NewTask(TaskTypePRJob, data)

	// Task options:
	// - TaskID: unique per webhook event (includes timestamp to allow multiple queued tasks per PR)
	// - MaxRetry: retry up to 3 times
	// - Queue: use default queue
	// Note: Concurrent execution is prevented by Redis lock in worker, not by task ID uniqueness
	// Using timestamp ensures that different event types (comments, reviews, etc.) can all be queued
	opts := []asynq.Option{
		asynq.TaskID(fmt.Sprintf("pr:%s:%d:%d", payload.Repository, payload.PRNumber, time.Now().UnixNano())),
		asynq.MaxRetry(3),
		asynq.Queue(QueueDefault),
	}

	info, err := c.asynq.EnqueueContext(ctx, task, opts...)
	if err != nil {
		return fmt.Errorf("failed to enqueue task: %w", err)
	}

	slog.Info("enqueued PR job",
		"task_id", info.ID,
		"repository", payload.Repository,
		"pr_number", payload.PRNumber,
		"sha", payload.SHA,
	)

	return nil
}

// Close closes the client connection
func (c *Client) Close() error {
	return c.asynq.Close()
}

// Lock represents a distributed lock for PR processing
type Lock struct {
	redis  *redis.Client
	key    string
	locked bool
}

// NewLock creates a new Redis-based lock for a specific PR
func NewLock(redisAddr, repository string, prNumber int) *Lock {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	return &Lock{
		redis: rdb,
		key:   fmt.Sprintf("pr:lock:%s:%d", repository, prNumber),
	}
}

// Acquire attempts to acquire the lock
func (l *Lock) Acquire(ctx context.Context) (bool, error) {
	// Try to set the key with NX (only if not exists) and expiration
	success, err := l.redis.SetNX(ctx, l.key, "locked", lockTTL).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	l.locked = success
	return success, nil
}

// Release releases the lock
func (l *Lock) Release(ctx context.Context) error {
	if !l.locked {
		return nil
	}

	if err := l.redis.Del(ctx, l.key).Err(); err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	l.locked = false
	return nil
}

// Close closes the Redis connection
func (l *Lock) Close() error {
	return l.redis.Close()
}
