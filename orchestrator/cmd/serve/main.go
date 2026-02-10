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
	"github.com/tom/laforge/orchestrator/internal/bashproxy"
	"github.com/tom/laforge/orchestrator/internal/config"
	"github.com/tom/laforge/orchestrator/internal/gitea"
	"github.com/tom/laforge/orchestrator/internal/notify"
	"github.com/tom/laforge/orchestrator/internal/proxy"
	"github.com/tom/laforge/orchestrator/internal/queue"
	"github.com/tom/laforge/orchestrator/internal/worker"
)

const (
	giteaSignatureHeader = "X-Gitea-Signature"
)

func main() {
	// Set up JSON structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	// Load configuration from file
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "/etc/laforge/config.yaml"
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

	// Build Anthropic proxy URL if token is configured
	var anthropicProxyURL string
	if cfg.Anthropic.APIKey != "" || cfg.Anthropic.OAuthToken != "" {
		anthropicProxyURL = "http://orchestrator:" + cfg.Anthropic.Port
	}

	// Initialize bash proxy token manager (30 minute timeout for tokens)
	bashProxyTokenManager := bashproxy.NewTokenManager(30 * time.Minute)

	// Initialize and start worker server in a goroutine
	workerServer := worker.NewServer(worker.Config{
		RedisAddr:             cfg.Redis.Address,
		Concurrency:           cfg.Worker.Concurrency,
		GiteaClient:           giteaClient,
		NotifyClient:          notifyClient,
		GiteaURL:              cfg.Gitea.URL,
		GiteaToken:            cfg.Gitea.Token,
		GitImage:              cfg.Docker.GitImage,
		BotUsername:           cfg.Bot.Username,
		BotEmail:              cfg.Bot.Email,
		NetworkName:           cfg.Docker.NetworkName,
		AnthropicProxyURL:     anthropicProxyURL,
		BashProxyTokenManager: bashProxyTokenManager,
		Repositories:          cfg.Repositories,
		LogsVolumeName:        cfg.Docker.LogsVolumeName,
	})
	workerServer.RegisterHandlers()

	go func() {
		slog.Info("starting worker server", "concurrency", cfg.Worker.Concurrency)
		if err := workerServer.Start(); err != nil {
			slog.Error("worker server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Start Anthropic proxy server if authentication is configured
	var proxyServer *http.Server
	if cfg.Anthropic.APIKey != "" || cfg.Anthropic.OAuthToken != "" {
		authType := "api_key"
		if cfg.Anthropic.OAuthToken != "" {
			authType = "oauth_token"
		}
		slog.Info("Anthropic API proxy enabled", "port", cfg.Anthropic.Port, "auth_type", authType)

		// Create proxy router
		proxyRouter := chi.NewRouter()
		proxyRouter.Use(middleware.RequestID)
		proxyRouter.Use(middleware.Logger)
		proxyRouter.Use(middleware.Recoverer)

		anthropicProxy := proxy.NewAnthropicProxy(cfg.Anthropic.APIKey, cfg.Anthropic.OAuthToken)
		proxyRouter.Post("/v1/messages", anthropicProxy.ServeHTTP)
		proxyRouter.Post("/v1/messages/count_tokens", anthropicProxy.ServeHTTP)

		// Start proxy server on separate port
		proxyAddr := ":" + cfg.Anthropic.Port
		proxyServer = &http.Server{Addr: proxyAddr, Handler: proxyRouter}

		go func() {
			slog.Info("starting Anthropic proxy server", "addr", proxyAddr)
			if err := proxyServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				slog.Error("proxy server failed", "error", err)
				os.Exit(1)
			}
		}()
	} else {
		slog.Warn("Anthropic API proxy disabled - no authentication configured")
	}

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

	// Bash proxy endpoint
	bashProxyHandler := bashproxy.NewHandler(bashProxyTokenManager, cfg.Docker.NetworkName)
	r.Post("/api/v1/bash", bashProxyHandler.HandleBashRequest)

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

	// Shutdown proxy server if it was started
	if proxyServer != nil {
		if err := proxyServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("proxy server shutdown failed", "error", err)
		}
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
		// Do NOT trigger on: synchronized (commits), closed, unassigned
		return action == "opened" || action == "reopened" || action == "assigned"

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

// checkBotAssignment checks if the bot is assigned to a PR
func checkBotAssignment(ctx context.Context, giteaClient *gitea.Client, botUsername, repository string, prNumber int) bool {
	assignees, err := giteaClient.GetPullRequestAssignees(ctx, repository, prNumber)
	if err != nil {
		slog.Error("failed to get PR assignees", "error", err, "repository", repository, "pr_number", prNumber)
		return false
	}

	for _, assignee := range assignees {
		if assignee == botUsername {
			return true
		}
	}
	return false
}

// parseCommentForSlashCommand parses a comment body for slash commands
// Returns: shouldActivate, promptType, modelName, commentBody
func parseCommentForSlashCommand(cfg *config.Config, commentBody string, botIsAssigned bool) (bool, string, string) {
	var promptType, modelName string
	hasSlashCommand := false

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

	return shouldActivate, promptType, modelName
}

func handleWebhook(cfg *config.Config, queueClient *queue.Client, giteaClient *gitea.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Read the request body with size limit to prevent DoS
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
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

		// Determine event type from X-Gitea-Event header
		eventType := r.Header.Get("X-Gitea-Event")

		// Parse webhook payload using SDK types based on event type
		switch eventType {
		case "pull_request":
			handlePullRequestEvent(ctx, cfg, queueClient, giteaClient, body, w)
		case "issue_comment":
			handleIssueCommentEvent(ctx, cfg, queueClient, giteaClient, body, w)
		case "pull_request_review":
			handlePullRequestReviewEvent(ctx, cfg, queueClient, giteaClient, body, w)
		case "pull_request_review_comment":
			handlePullRequestReviewCommentEvent(ctx, cfg, queueClient, giteaClient, body, w)
		default:
			slog.Debug("ignoring unsupported event type", "event", eventType)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "ignored",
			})
		}
	}
}

// handlePullRequestEvent processes pull_request webhook events using SDK types
func handlePullRequestEvent(ctx context.Context, cfg *config.Config, queueClient *queue.Client, giteaClient *gitea.Client, body []byte, w http.ResponseWriter) {
	var payload gitea.PullRequestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Error("failed to parse pull_request payload", "error", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Log the webhook event
	slog.Info("webhook received",
		"event", "pull_request",
		"action", payload.Action,
		"repository", payload.Repository.FullName,
		"number", payload.Index,
		"sender", payload.Sender.UserName,
	)

	// Check if this event should trigger an agent run
	if !shouldTriggerAgent("pull_request", string(payload.Action), payload.Sender.UserName, cfg.Bot.Username) {
		slog.Debug("event filtered out, not triggering agent",
			"event", "pull_request",
			"action", payload.Action,
			"sender", payload.Sender.UserName,
		)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ignored",
		})
		return
	}

	// Check if agent should be activated (bot assigned or slash command present)
	botIsAssigned := checkBotAssignment(ctx, giteaClient, cfg.Bot.Username, payload.Repository.FullName, int(payload.Index))
	shouldActivate, promptType, modelName := parseCommentForSlashCommand(cfg, "", botIsAssigned)

	if !shouldActivate {
		slog.Info("agent not activated - bot not assigned and no slash command found",
			"event", "pull_request",
			"repository", payload.Repository.FullName,
			"pr_number", payload.Index,
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
		"event", "pull_request",
		"repository", payload.Repository.FullName,
		"pr_number", payload.Index,
		"prompt_type", promptType,
		"model", fullModelID,
	)

	// Extract PR details from SDK payload
	headRepo := payload.Repository.FullName
	if payload.PullRequest.Head != nil && payload.PullRequest.Head.Repository != nil {
		headRepo = payload.PullRequest.Head.Repository.FullName
	}

	headSHA := ""
	branch := ""
	if payload.PullRequest.Head != nil {
		headSHA = payload.PullRequest.Head.Sha
		branch = payload.PullRequest.Head.Ref
	}

	enqueueJob(ctx, cfg, queueClient, giteaClient, w, queue.PRJobPayload{
		Repository:      payload.Repository.FullName,
		PRNumber:        int(payload.Index),
		SHA:             headSHA,
		Action:          string(payload.Action),
		Sender:          payload.Sender.UserName,
		HeadRepository:  headRepo,
		Branch:          branch,
		PromptType:      promptType,
		Model:           fullModelID,
		ModelImage:      modelImage,
		IsCleanupAction: promptType == "cleanup",
	})
}

// handleIssueCommentEvent processes issue_comment webhook events using SDK types
func handleIssueCommentEvent(ctx context.Context, cfg *config.Config, queueClient *queue.Client, giteaClient *gitea.Client, body []byte, w http.ResponseWriter) {
	var payload gitea.IssueCommentPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Error("failed to parse issue_comment payload", "error", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Log the webhook event
	slog.Info("webhook received",
		"event", "issue_comment",
		"action", payload.Action,
		"repository", payload.Repository.FullName,
		"number", payload.Issue.Index,
		"sender", payload.Sender.UserName,
	)

	// Check if this is a PR comment (not a regular issue)
	if !payload.IsPull {
		slog.Debug("issue_comment on regular issue, not PR - ignoring")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ignored"})
		return
	}

	// Check if this event should trigger an agent run
	if !shouldTriggerAgent("issue_comment", string(payload.Action), payload.Sender.UserName, cfg.Bot.Username) {
		slog.Debug("event filtered out, not triggering agent",
			"event", "issue_comment",
			"action", payload.Action,
			"sender", payload.Sender.UserName,
		)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ignored",
		})
		return
	}

	// Check if agent should be activated (bot assigned or slash command present)
	botIsAssigned := checkBotAssignment(ctx, giteaClient, cfg.Bot.Username, payload.Repository.FullName, int(payload.Issue.Index))
	commentBody := ""
	if payload.Comment != nil {
		commentBody = payload.Comment.Body
	}
	shouldActivate, promptType, modelName := parseCommentForSlashCommand(cfg, commentBody, botIsAssigned)

	if !shouldActivate {
		slog.Info("agent not activated - bot not assigned and no slash command found",
			"event", "issue_comment",
			"repository", payload.Repository.FullName,
			"pr_number", payload.Issue.Index,
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
		"event", "issue_comment",
		"repository", payload.Repository.FullName,
		"pr_number", payload.Issue.Index,
		"prompt_type", promptType,
		"model", fullModelID,
		"has_comment", commentBody != "",
	)

	// Fetch PR details from Gitea API to get head SHA and branch
	pr, err := giteaClient.GetPullRequest(ctx, payload.Repository.FullName, int(payload.Issue.Index))
	if err != nil {
		slog.Error("failed to fetch PR details for issue_comment",
			"error", err,
			"repository", payload.Repository.FullName,
			"pr_number", payload.Issue.Index,
		)
		http.Error(w, "Failed to fetch PR details", http.StatusInternalServerError)
		return
	}

	enqueueJob(ctx, cfg, queueClient, giteaClient, w, queue.PRJobPayload{
		Repository:      payload.Repository.FullName,
		PRNumber:        int(payload.Issue.Index),
		SHA:             pr.HeadSHA,
		Action:          string(payload.Action),
		Sender:          payload.Sender.UserName,
		HeadRepository:  pr.HeadRepo,
		Branch:          pr.Branch,
		PromptType:      promptType,
		Model:           fullModelID,
		ModelImage:      modelImage,
		IsCleanupAction: promptType == "cleanup",
	})
}

// handlePullRequestReviewEvent processes pull_request_review webhook events using SDK types
func handlePullRequestReviewEvent(ctx context.Context, cfg *config.Config, queueClient *queue.Client, giteaClient *gitea.Client, body []byte, w http.ResponseWriter) {
	var payload gitea.PullRequestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Error("failed to parse pull_request_review payload", "error", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Log the webhook event
	slog.Info("webhook received",
		"event", "pull_request_review",
		"action", payload.Action,
		"repository", payload.Repository.FullName,
		"number", payload.Index,
		"sender", payload.Sender.UserName,
	)

	// Check if this event should trigger an agent run
	if !shouldTriggerAgent("pull_request_review", string(payload.Action), payload.Sender.UserName, cfg.Bot.Username) {
		slog.Debug("event filtered out, not triggering agent",
			"event", "pull_request_review",
			"action", payload.Action,
			"sender", payload.Sender.UserName,
		)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ignored",
		})
		return
	}

	// Check if agent should be activated (bot assigned or slash command present)
	botIsAssigned := checkBotAssignment(ctx, giteaClient, cfg.Bot.Username, payload.Repository.FullName, int(payload.Index))
	reviewBody := ""
	if payload.Review != nil {
		reviewBody = payload.Review.Content
	}
	shouldActivate, promptType, modelName := parseCommentForSlashCommand(cfg, reviewBody, botIsAssigned)

	if !shouldActivate {
		slog.Info("agent not activated - bot not assigned and no slash command found",
			"event", "pull_request_review",
			"repository", payload.Repository.FullName,
			"pr_number", payload.Index,
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
		"event", "pull_request_review",
		"repository", payload.Repository.FullName,
		"pr_number", payload.Index,
		"prompt_type", promptType,
		"model", fullModelID,
		"has_comment", reviewBody != "",
	)

	// Extract PR details from SDK payload
	headRepo := payload.Repository.FullName
	if payload.PullRequest.Head != nil && payload.PullRequest.Head.Repository != nil {
		headRepo = payload.PullRequest.Head.Repository.FullName
	}

	headSHA := ""
	branch := ""
	if payload.PullRequest.Head != nil {
		headSHA = payload.PullRequest.Head.Sha
		branch = payload.PullRequest.Head.Ref
	}

	enqueueJob(ctx, cfg, queueClient, giteaClient, w, queue.PRJobPayload{
		Repository:      payload.Repository.FullName,
		PRNumber:        int(payload.Index),
		SHA:             headSHA,
		Action:          string(payload.Action),
		Sender:          payload.Sender.UserName,
		HeadRepository:  headRepo,
		Branch:          branch,
		PromptType:      promptType,
		Model:           fullModelID,
		ModelImage:      modelImage,
		IsCleanupAction: promptType == "cleanup",
	})
}

// handlePullRequestReviewCommentEvent processes pull_request_review_comment webhook events using SDK types
func handlePullRequestReviewCommentEvent(ctx context.Context, cfg *config.Config, queueClient *queue.Client, giteaClient *gitea.Client, body []byte, w http.ResponseWriter) {
	var payload gitea.PullRequestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		slog.Error("failed to parse pull_request_review_comment payload", "error", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Log the webhook event
	slog.Info("webhook received",
		"event", "pull_request_review_comment",
		"action", payload.Action,
		"repository", payload.Repository.FullName,
		"number", payload.Index,
		"sender", payload.Sender.UserName,
	)

	// Check if this event should trigger an agent run
	if !shouldTriggerAgent("pull_request_review_comment", string(payload.Action), payload.Sender.UserName, cfg.Bot.Username) {
		slog.Debug("event filtered out, not triggering agent",
			"event", "pull_request_review_comment",
			"action", payload.Action,
			"sender", payload.Sender.UserName,
		)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ignored",
		})
		return
	}

	// Check if agent should be activated (bot assigned or slash command present)
	botIsAssigned := checkBotAssignment(ctx, giteaClient, cfg.Bot.Username, payload.Repository.FullName, int(payload.Index))
	commentBody := ""
	if payload.Review != nil && payload.Review.Content != "" {
		commentBody = payload.Review.Content
	}
	shouldActivate, promptType, modelName := parseCommentForSlashCommand(cfg, commentBody, botIsAssigned)

	if !shouldActivate {
		slog.Info("agent not activated - bot not assigned and no slash command found",
			"event", "pull_request_review_comment",
			"repository", payload.Repository.FullName,
			"pr_number", payload.Index,
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
		"event", "pull_request_review_comment",
		"repository", payload.Repository.FullName,
		"pr_number", payload.Index,
		"prompt_type", promptType,
		"model", fullModelID,
		"has_comment", commentBody != "",
	)

	// Extract PR details from SDK payload
	headRepo := payload.Repository.FullName
	if payload.PullRequest.Head != nil && payload.PullRequest.Head.Repository != nil {
		headRepo = payload.PullRequest.Head.Repository.FullName
	}

	headSHA := ""
	branch := ""
	if payload.PullRequest.Head != nil {
		headSHA = payload.PullRequest.Head.Sha
		branch = payload.PullRequest.Head.Ref
	}

	enqueueJob(ctx, cfg, queueClient, giteaClient, w, queue.PRJobPayload{
		Repository:      payload.Repository.FullName,
		PRNumber:        int(payload.Index),
		SHA:             headSHA,
		Action:          string(payload.Action),
		Sender:          payload.Sender.UserName,
		HeadRepository:  headRepo,
		Branch:          branch,
		PromptType:      promptType,
		Model:           fullModelID,
		ModelImage:      modelImage,
		IsCleanupAction: promptType == "cleanup",
	})
}

// enqueueJob creates and enqueues a PR job to the queue
func enqueueJob(ctx context.Context, cfg *config.Config, queueClient *queue.Client, giteaClient *gitea.Client, w http.ResponseWriter, jobPayload queue.PRJobPayload) {
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
