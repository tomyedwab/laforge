package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	defaultPort          = "8080"
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
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	webhookSecret := os.Getenv("WEBHOOK_SECRET")
	if webhookSecret == "" {
		slog.Warn("WEBHOOK_SECRET not set, webhook signature validation disabled")
	}

	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Routes
	r.Get("/health", handleHealth)
	r.Post("/webhook", handleWebhook(webhookSecret))

	addr := ":" + port
	slog.Info("starting orchestrator server", "addr", addr)

	if err := http.ListenAndServe(addr, r); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"healthy": true})
}

func handleWebhook(secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("failed to read request body", "error", err)
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Validate signature if secret is configured
		if secret != "" {
			signature := r.Header.Get(giteaSignatureHeader)
			if signature == "" {
				slog.Warn("webhook received without signature")
				http.Error(w, "Missing signature", http.StatusUnauthorized)
				return
			}

			if !validateSignature(body, signature, secret) {
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

		// TODO: Add actual webhook handling logic here
		// For now, just log and acknowledge receipt

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

// The following webhook event types are relevant based on .gitea/workflows/agent.yaml:
// - pull_request (actions: opened, reopened, assigned)
// - pull_request_review (actions: submitted, edited)
// - pull_request_review_comment (actions: created, edited)
// - issue_comment (actions: created, edited - on PRs only)
//
// Event handling will be implemented in future PRs. For now we just log.
