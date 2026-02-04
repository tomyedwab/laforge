package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/tom/laforge/orchestrator/internal/config"
	"github.com/tom/laforge/orchestrator/internal/gitea"
	"github.com/tom/laforge/orchestrator/internal/notify"
	"github.com/tom/laforge/orchestrator/internal/queue"
	"github.com/tom/laforge/orchestrator/internal/worker"
)

const (
	giteaSignatureHeader = "X-Gitea-Signature"
)

// GiteaWebhookPayload represents a generic Gitea webhook payload
type GiteaWebhookPayload struct {
	Action      string          `json:"action"`
	Number      int             `json:"number"`
	PullRequest json.RawMessage `json:"pull_request,omitempty"`
	Issue       json.RawMessage `json:"issue,omitempty"`
	Comment     json.RawMessage `json:"comment,omitempty"`
	Review      json.RawMessage `json:"review,omitempty"`
	Repository  struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

func main() {
	// Set up JSON structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	// Load configuration from file
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "/etc/laforge/laforge-config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load configuration", "error", err, "path", configPath)
		os.Exit(1)
	}

	slog.Info("configuration loaded successfully", "path", configPath)

	// Log warnings for optional fields
	if cfg.Server.WebhookSecret == "" {
		slog.Warn("webhook_secret not set, webhook signature validation disabled")
	}
	if cfg.Gitea.Token == "" {
		slog.Warn("gitea.token not set, commit status updates will fail")
	}

	// Initialize Gitea client
	giteaClient, err := gitea.NewClient(cfg.Gitea.URL, cfg.Gitea.Token)
	if err != nil {
		slog.Error("failed to create Gitea client", "error", err)
		os.Exit(1)
	}

	// Initialize notification client
	notifyClient := notify.NewClient(notify.Config{
		Endpoint: cfg.Notifications.NtfyEndpoint,
		GiteaURL: cfg.Gitea.ExternalURL,
	})

	// Initialize queue client
	queueClient := queue.NewClient(cfg.Redis.Address)
	defer queueClient.Close()

	// Initialize and start worker server in a goroutine
	workerServer := worker.NewServer(worker.Config{
		RedisAddr:    cfg.Redis.Address,
		Concurrency:  cfg.Worker.Concurrency,
		GiteaClient:  giteaClient,
		NotifyClient: notifyClient,
		GiteaURL:     cfg.Gitea.URL,
		GiteaToken:   cfg.Gitea.Token,
		GitImage:     cfg.Docker.GitImage,
		BotUsername:  cfg.Bot.Username,
		BotEmail:     cfg.Bot.Email,
	})
	workerServer.RegisterHandlers()

	go func() {
		slog.Info("starting worker server", "concurrency", cfg.Worker.Concurrency)
		if err := workerServer.Start(); err != nil {
			slog.Error("worker server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Set up HTTP router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Routes
	r.Get("/health", handleHealth)
	r.Post("/webhook", handleWebhook(cfg, queueClient, giteaClient))

	// Set up graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start HTTP server in a goroutine
	addr := ":" + cfg.Server.Port
	server := &http.Server{Addr: addr, Handler: r}

	go func() {
		slog.Info("starting orchestrator server", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()
	slog.Info("shutting down gracefully")

	// Shutdown worker server
	workerServer.Shutdown()

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown failed", "error", err)
	}

	slog.Info("server stopped")
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"healthy": true})
}

// parseSlashCommand extracts slash command from comment body
// Format: /<prompt-type> <model> OR /cleanup
// Examples: /plan sonnet, /critique opus, /implement haiku, /cleanup
// Returns: promptType, modelName, found
func parseSlashCommand(cfg *config.Config, commentBody string) (string, string, bool) {
	// Check for /cleanup command first (takes precedence)
	if strings.Contains(commentBody, "/cleanup") {
		return "cleanup", "", true
	}

	// Match /<word> <word> pattern
	// Allowed prompt types: implement, plan, critique
	for _, promptType := range cfg.Prompts.ValidTypes {
		if idx := strings.Index(commentBody, "/"+promptType); idx != -1 {
			// Extract the model name after the prompt type
			remaining := commentBody[idx:]
			parts := strings.Fields(remaining)
			if len(parts) >= 2 {
				modelName := parts[1]
				// Validate model name
				if _, ok := cfg.Models[modelName]; ok {
					return promptType, modelName, true
				}
			}
		}
	}
	return "", "", false
}

// resolvePromptTypeAndModel resolves prompt type and model to full values
// Returns: promptType, fullModelID, modelImage
func resolvePromptTypeAndModel(cfg *config.Config, promptType, modelName string) (string, string, string) {
	// Default values
	if promptType == "" {
		promptType = cfg.Prompts.DefaultType
	}
	if modelName == "" {
		modelName = cfg.Prompts.DefaultModel
	}

	// Resolve model name to full model configuration
	model := cfg.GetModel(modelName)

	return promptType, model.ModelID, model.Image
}

// shouldTriggerAgent determines if a webhook event should trigger an agent run
// based on the event type, action, and sender.
//
// Agent runs are triggered by:
// - pull_request: opened, reopened, assigned
// - issue_comment: created, edited (on PRs only, filtered later)
// - pull_request_review: submitted
// - pull_request_review_comment: created, edited
//
// Agent runs are NOT triggered by:
// - Events from the bot itself (prevents infinite loops)
// - pull_request: synchronized (commits), closed, edited, unassigned
func shouldTriggerAgent(eventType, action, sender, botUsername string) bool {
	// Never trigger if the sender is the bot itself
	if sender == botUsername {
		return false
	}

	// Check event type and action combinations
	switch eventType {
	case "pull_request":
		// Trigger on: opened, reopened, assigned
		// Do NOT trigger on: synchronized (commits), closed, edited, unassigned
		return action == "opened" || action == "reopened" || action == "assigned" || action == "edited"

	case "issue_comment":
		// Trigger on: created, edited
		// Note: We'll verify it's actually a PR comment in the handler
		return action == "created" || action == "edited"

	case "pull_request_review":
		// Trigger on: submitted (all review types: approved, changes_requested, commented)
		return action == "submitted"

	case "pull_request_review_comment":
		// Trigger on: created, edited
		return action == "created" || action == "edited"

	default:
		// Unknown event type, don't trigger
		return false
	}
}

// shouldActivateAgent determines whether to activate the agent based on:
// 1. Bot is assigned to the PR
// 2. A new comment contains a slash command
// Returns: shouldActivate, promptType, modelName, commentBody (for logging)
func shouldActivateAgent(ctx context.Context, cfg *config.Config, giteaClient *gitea.Client, eventType string, payload GiteaWebhookPayload) (bool, string, string, string) {
	botUsername := cfg.Bot.Username
	repository := payload.Repository.FullName
	prNumber := payload.Number

	// Check if bot is assigned
	assignees, err := giteaClient.GetPullRequestAssignees(ctx, repository, prNumber)
	if err != nil {
		slog.Error("failed to get PR assignees", "error", err, "repository", repository, "pr_number", prNumber)
		// Don't fail the webhook, just log and continue
		assignees = []string{}
	}

	botIsAssigned := false
	for _, assignee := range assignees {
		if assignee == botUsername {
			botIsAssigned = true
			break
		}
	}

	// Check for slash command in comments
	var commentBody string
	hasSlashCommand := false
	var promptType, modelName string

	// Extract comment body based on event type
	switch eventType {
	case "issue_comment":
		if payload.Comment != nil {
			var comment struct {
				Body string `json:"body"`
			}
			if err := json.Unmarshal(payload.Comment, &comment); err == nil {
				commentBody = comment.Body
			}
		}
	case "pull_request_review":
		if payload.Review != nil {
			var review struct {
				Body string `json:"body"`
			}
			if err := json.Unmarshal(payload.Review, &review); err == nil {
				commentBody = review.Body
			}
		}
	case "pull_request_review_comment":
		if payload.Comment != nil {
			var comment struct {
				Body string `json:"body"`
			}
			if err := json.Unmarshal(payload.Comment, &comment); err == nil {
				commentBody = comment.Body
			}
		}
	}

	// Parse slash command if we have a comment body
	if commentBody != "" {
		promptType, modelName, hasSlashCommand = parseSlashCommand(cfg, commentBody)
	}

	// Activation logic:
	// - If bot is assigned AND slash command present: activate with slash command's prompt/model
	// - If only bot is assigned (no slash command): activate with defaults
	// - If only slash command (bot not assigned): activate with slash command's prompt/model
	// - If neither: don't activate
	shouldActivate := botIsAssigned || hasSlashCommand

	// If activated but no slash command, use defaults
	if shouldActivate && !hasSlashCommand {
		promptType = cfg.Prompts.DefaultType
		modelName = cfg.Prompts.DefaultModel
	}

	return shouldActivate, promptType, modelName, commentBody
}

func handleWebhook(cfg *config.Config, queueClient *queue.Client, giteaClient *gitea.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Read the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("failed to read request body", "error", err)
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Validate signature if secret is configured
		if cfg.Server.WebhookSecret != "" {
			signature := r.Header.Get(giteaSignatureHeader)
			if signature == "" {
				slog.Warn("webhook received without signature")
				http.Error(w, "Missing signature", http.StatusUnauthorized)
				return
			}

			if !validateSignature(body, signature, cfg.Server.WebhookSecret) {
				slog.Warn("webhook signature validation failed")
				http.Error(w, "Invalid signature", http.StatusUnauthorized)
				return
			}
		}

		// Parse the webhook payload
		var payload GiteaWebhookPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			slog.Error("failed to parse webhook payload", "error", err)
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		// Determine event type from X-Gitea-Event header
		eventType := r.Header.Get("X-Gitea-Event")

		// Log the webhook event
		slog.Info("webhook received",
			"event", eventType,
			"action", payload.Action,
			"repository", payload.Repository.FullName,
			"number", payload.Number,
			"sender", payload.Sender.Login,
		)

		// Check if this event should trigger an agent run
		if !shouldTriggerAgent(eventType, payload.Action, payload.Sender.Login, cfg.Bot.Username) {
			slog.Debug("event filtered out, not triggering agent",
				"event", eventType,
				"action", payload.Action,
				"sender", payload.Sender.Login,
			)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "ignored",
			})
			return
		}

		// Check if agent should be activated (bot assigned or slash command present)
		shouldActivate, promptType, modelName, commentBody := shouldActivateAgent(ctx, cfg, giteaClient, eventType, payload)
		if !shouldActivate {
			slog.Info("agent not activated - bot not assigned and no slash command found",
				"event", eventType,
				"repository", payload.Repository.FullName,
				"pr_number", payload.Number,
			)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "not_activated",
			})
			return
		}

		// Resolve prompt type and model to full values
		promptType, fullModelID, modelImage := resolvePromptTypeAndModel(cfg, promptType, modelName)

		slog.Info("agent activated",
			"event", eventType,
			"repository", payload.Repository.FullName,
			"pr_number", payload.Number,
			"prompt_type", promptType,
			"model", fullModelID,
			"has_comment", commentBody != "",
		)

		// Extract PR information based on event type
		var prNumber int
		var headSHA string
		var headRepo string
		var branch string

		switch eventType {
		case "pull_request":
			if payload.PullRequest == nil {
				slog.Error("pull_request event missing pull_request field")
				http.Error(w, "Invalid pull request event", http.StatusBadRequest)
				return
			}
			// Extract PR details
			var pr struct {
				Number int `json:"number"`
				Head   struct {
					SHA  string `json:"sha"`
					Ref  string `json:"ref"`
					Repo struct {
						FullName string `json:"full_name"`
					} `json:"repo"`
				} `json:"head"`
			}
			if err := json.Unmarshal(payload.PullRequest, &pr); err != nil {
				slog.Error("failed to parse pull request data", "error", err)
				http.Error(w, "Invalid pull request data", http.StatusBadRequest)
				return
			}
			prNumber = pr.Number
			headSHA = pr.Head.SHA
			branch = pr.Head.Ref
			headRepo = pr.Head.Repo.FullName
			if headRepo == "" {
				headRepo = payload.Repository.FullName
			}

		case "issue_comment":
			// For issue_comment events, verify it's on a PR (not an issue)
			if payload.Issue == nil {
				slog.Error("issue_comment event missing issue field")
				http.Error(w, "Invalid issue comment event", http.StatusBadRequest)
				return
			}
			var issue struct {
				Number      int `json:"number"`
				PullRequest *struct {
					URL string `json:"url"`
				} `json:"pull_request"`
			}
			if err := json.Unmarshal(payload.Issue, &issue); err != nil {
				slog.Error("failed to parse issue data", "error", err)
				http.Error(w, "Invalid issue data", http.StatusBadRequest)
				return
			}
			// If pull_request field is null, this is a regular issue, not a PR
			if issue.PullRequest == nil {
				slog.Debug("issue_comment on regular issue, not PR - ignoring")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "ignored"})
				return
			}
			prNumber = issue.Number
			// For comments, we need to fetch the PR HEAD to get the current SHA
			// For now, we'll use a placeholder and fetch it via API
			// TODO: Consider fetching PR details via Gitea API here
			headRepo = payload.Repository.FullName
			headSHA = "" // Will be filled by fetching PR details

		case "pull_request_review", "pull_request_review_comment":
			if payload.PullRequest == nil {
				slog.Error("review event missing pull_request field")
				http.Error(w, "Invalid review event", http.StatusBadRequest)
				return
			}
			var pr struct {
				Number int `json:"number"`
				Head   struct {
					SHA  string `json:"sha"`
					Ref  string `json:"ref"`
					Repo struct {
						FullName string `json:"full_name"`
					} `json:"repo"`
				} `json:"head"`
			}
			if err := json.Unmarshal(payload.PullRequest, &pr); err != nil {
				slog.Error("failed to parse pull request data", "error", err)
				http.Error(w, "Invalid pull request data", http.StatusBadRequest)
				return
			}
			prNumber = pr.Number
			headSHA = pr.Head.SHA
			branch = pr.Head.Ref
			headRepo = pr.Head.Repo.FullName
			if headRepo == "" {
				headRepo = payload.Repository.FullName
			}

		default:
			// This shouldn't happen if shouldTriggerAgent is working correctly
			slog.Error("unexpected event type after filtering", "event", eventType)
			http.Error(w, "Unsupported event type", http.StatusBadRequest)
			return
		}

		// For issue_comment events, we need to fetch the PR details including head SHA and branch
		if eventType == "issue_comment" {
			// Fetch PR details from Gitea API
			pr, err := giteaClient.GetPullRequest(ctx, payload.Repository.FullName, prNumber)
			if err != nil {
				slog.Error("failed to fetch PR details for issue_comment",
					"error", err,
					"repository", payload.Repository.FullName,
					"pr_number", prNumber,
				)
				http.Error(w, "Failed to fetch PR details", http.StatusInternalServerError)
				return
			}
			headSHA = pr.HeadSHA
			headRepo = pr.HeadRepo
			branch = pr.Branch
		}

		// Create job payload
		jobPayload := queue.PRJobPayload{
			Repository:      payload.Repository.FullName,
			PRNumber:        prNumber,
			SHA:             headSHA,
			Action:          payload.Action,
			Sender:          payload.Sender.Login,
			HeadRepository:  headRepo,
			Branch:          branch,
			PromptType:      promptType,
			Model:           fullModelID,
			ModelImage:      modelImage,
			IsCleanupAction: promptType == "cleanup",
		}

		// Update Gitea status to "pending" (queued)
		if err := giteaClient.UpdateStatus(ctx, jobPayload.HeadRepository, jobPayload.SHA, gitea.StatusPending); err != nil {
			slog.Error("failed to update status to pending",
				"error", err,
				"repository", jobPayload.Repository,
				"head_repository", jobPayload.HeadRepository,
				"sha", jobPayload.SHA,
			)
			// Continue even if status update fails
		}

		// Enqueue the job
		if err := queueClient.EnqueuePRJob(ctx, jobPayload); err != nil {
			slog.Error("failed to enqueue job", "error", err)
			http.Error(w, "Failed to enqueue job", http.StatusInternalServerError)
			return
		}

		slog.Info("PR job enqueued",
			"event", eventType,
			"repository", jobPayload.Repository,
			"pr_number", jobPayload.PRNumber,
			"sha", jobPayload.SHA,
			"head_repository", jobPayload.HeadRepository,
			"prompt_type", jobPayload.PromptType,
			"model", jobPayload.Model,
		)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "received",
		})
	}
}

// validateSignature validates the Gitea webhook signature
// Gitea uses HMAC-SHA256 for webhook signatures
func validateSignature(payload []byte, signature, secret string) bool {
	// Gitea signature format is just the hex-encoded HMAC
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	// Constant time comparison to prevent timing attacks
	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}

// Event filtering is implemented in shouldTriggerAgent() function above.
// The following webhook event types trigger agent runs:
// - pull_request (actions: opened, reopened, assigned)
//   - Does NOT trigger on: synchronized (commits), closed, edited, unassigned
// - pull_request_review (actions: submitted - all review types)
// - pull_request_review_comment (actions: created, edited)
// - issue_comment (actions: created, edited - on PRs only, not issues)
//
// Events from the bot user (BOT_USERNAME) are automatically filtered out to prevent loops.
