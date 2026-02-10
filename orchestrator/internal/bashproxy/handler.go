package bashproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/tom/laforge/orchestrator/internal/docker"
	"github.com/tom/laforge/orchestrator/internal/types"
)

// Handler handles bash proxy requests
type Handler struct {
	tokenManager *TokenManager
	networkName  string
	giteaClient  GiteaClient
}

// GiteaClient defines the interface for Gitea operations needed by the handler
type GiteaClient interface {
	EditComment(ctx context.Context, repository string, commentID int64, body string) error
}

// NewHandler creates a new bash proxy handler
func NewHandler(tokenManager *TokenManager, networkName string, giteaClient GiteaClient) *Handler {
	return &Handler{
		tokenManager: tokenManager,
		networkName:  networkName,
		giteaClient:  giteaClient,
	}
}

// HandleBashRequest handles POST /api/v1/bash requests
func (h *Handler) HandleBashRequest(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract and validate the bearer token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
		return
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
		return
	}

	// Validate the token and get job context
	jobCtx, valid := h.tokenManager.ValidateToken(token)
	if !valid {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	// Parse the request body
	var req types.BashRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.Command == "" {
		http.Error(w, "Command is required", http.StatusBadRequest)
		return
	}

	// Check if this is an update_status command
	if h.isUpdateStatusCommand(req.Command) {
		h.handleUpdateStatus(w, r, token, jobCtx, req.Command)
		return
	}

	slog.Info("executing bash command in devcontainer",
		"repository", jobCtx.Repository,
		"volume", jobCtx.VolumeName,
		"image", jobCtx.DevcontainerImage,
		"command_length", len(req.Command),
	)

	// Execute the command in the devcontainer
	stdout, stderr, exitCode, err := h.executeCommand(r.Context(), jobCtx, req.Command)
	if err != nil {
		slog.Error("failed to execute command in devcontainer", "error", err)
		http.Error(w, fmt.Sprintf("Failed to execute command: %v", err), http.StatusInternalServerError)
		return
	}

	// Return the response
	response := types.BashResponse{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode response", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	slog.Info("bash command completed",
		"repository", jobCtx.Repository,
		"exit_code", exitCode,
	)
}

// executeCommand executes a bash command in the devcontainer using docker exec
func (h *Handler) executeCommand(ctx context.Context, jobCtx *types.BashJobContext, command string) (string, string, int, error) {
	// Validate that we have a devcontainer ID
	if jobCtx.DevcontainerID == "" {
		return "", "", 1, fmt.Errorf("devcontainer ID is empty - devcontainer not started")
	}

	// Create a context with timeout
	timeout := time.Duration(jobCtx.CommandTimeout) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute // default timeout
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create a Docker client for this command execution
	// We use a unique job name to avoid conflicts (though we won't create containers)
	jobName := fmt.Sprintf("%s-bash-%d", jobCtx.VolumeName, time.Now().UnixNano())
	dockerClient, err := docker.NewClient(jobName)
	if err != nil {
		return "", "", 1, fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer dockerClient.Close()

	// Execute the command in the running devcontainer using docker exec
	stdout, stderr, exitCode, err := dockerClient.ExecDevcontainerCommand(
		execCtx,
		jobCtx.DevcontainerID,
		command,
	)
	if err != nil {
		return "", "", 1, fmt.Errorf("failed to exec command in devcontainer: %w", err)
	}

	return stdout, stderr, exitCode, nil
}

// isUpdateStatusCommand checks if a command contains "update_status"
// This is a simple substring search - if the command contains "update_status" anywhere,
// we treat it as an update_status command and intercept it.
func (h *Handler) isUpdateStatusCommand(command string) bool {
	return strings.Contains(command, "update_status")
}

// StatusUpdateRequest represents the JSON payload for update_status
type StatusUpdateRequest struct {
	Message string `json:"message"`
}

// extractUpdateStatusMessage extracts and parses the message from an update_status command.
// Handles two formats:
// 1. Simple: update_status '{"message": "text"}' (direct JSON)
// 2. Claude Code: eval "update_status '{\"message\": \"text\"}'" (double-encoded JSON)
func extractUpdateStatusMessage(command string) (string, error) {
	statusIdx := strings.Index(command, "update_status")
	if statusIdx == -1 {
		return "", fmt.Errorf("update_status not found in command")
	}

	afterUpdateStatus := command[statusIdx+len("update_status"):]
	afterUpdateStatus = strings.TrimSpace(afterUpdateStatus)

	if len(afterUpdateStatus) == 0 {
		return "", fmt.Errorf("no parameter after update_status")
	}

	quoteChar := afterUpdateStatus[0]
	if quoteChar != '"' && quoteChar != '\'' {
		return "", fmt.Errorf("expected quoted parameter after update_status")
	}

	// Find the end of the JSON by looking for the closing brace followed by the quote
	// This avoids issues with apostrophes in the message (e.g., "we'll")
	endPattern := "}" + string(quoteChar)
	endIdx := strings.Index(afterUpdateStatus, endPattern)
	if endIdx == -1 {
		return "", fmt.Errorf("could not find end of JSON parameter")
	}

	// Extract the content between quotes (including the closing brace)
	jsonContent := afterUpdateStatus[1 : endIdx+1]

	// Try approach 1: Parse directly as JSON (simple format)
	var statusReq StatusUpdateRequest
	if err := json.Unmarshal([]byte(jsonContent), &statusReq); err == nil {
		return strings.TrimSpace(statusReq.Message), nil
	}

	// Try approach 2: Double-parse for escaped JSON (Claude Code format)
	// Step 1: Parse as a JSON string to decode escape sequences
	var jsonDecoded string
	if err := json.Unmarshal([]byte(`"`+jsonContent+`"`), &jsonDecoded); err != nil {
		return "", fmt.Errorf("failed to decode JSON string: %w (input: %q)", err, jsonContent)
	}

	// Step 2: Parse the decoded JSON object to extract the message field
	if err := json.Unmarshal([]byte(jsonDecoded), &statusReq); err != nil {
		return "", fmt.Errorf("failed to parse JSON object: %w (decoded: %q)", err, jsonDecoded)
	}

	return strings.TrimSpace(statusReq.Message), nil
}

// handleUpdateStatus processes an update_status command
func (h *Handler) handleUpdateStatus(w http.ResponseWriter, r *http.Request, token string, jobCtx *types.BashJobContext, command string) {
	message, err := extractUpdateStatusMessage(command)
	if err != nil {
		slog.Warn("failed to extract update_status message",
			"error", err,
			"command", command,
		)
		// Return success anyway to not break the agent
		response := types.BashResponse{
			Stdout:   "",
			Stderr:   "",
			ExitCode: 0,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	if message == "" {
		slog.Warn("update_status called with empty message")
		// Return success anyway
		response := types.BashResponse{
			Stdout:   "",
			Stderr:   "",
			ExitCode: 0,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	slog.Info("processing update_status command",
		"repository", jobCtx.Repository,
		"pr_number", jobCtx.PRNumber,
		"message", message,
	)

	// Get the status tracker for this token
	statusTracker := h.tokenManager.GetStatusTracker(token)
	if statusTracker == nil {
		slog.Warn("no status tracker found for token")
		// Return success anyway - log the issue but don't fail the command
		response := types.BashResponse{
			Stdout:   "",
			Stderr:   "",
			ExitCode: 0,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Add the message to the tracker
	statusTracker.AddMessage(message)

	// Build the updated comment body
	commentBody := statusTracker.BuildCommentBody()

	// Update the live comment if we have a comment ID
	if jobCtx.LiveCommentID > 0 && h.giteaClient != nil {
		if err := h.giteaClient.EditComment(r.Context(), jobCtx.Repository, int64(jobCtx.LiveCommentID), commentBody); err != nil {
			// Log the error but don't fail - the PR author said not to return errors
			slog.Error("failed to update live status comment", "error", err, "comment_id", jobCtx.LiveCommentID)
		} else {
			slog.Info("updated live status comment", "comment_id", jobCtx.LiveCommentID, "message_count", statusTracker.GetMessageCount())
		}
	}

	// Return success response
	response := types.BashResponse{
		Stdout:   "",
		Stderr:   "",
		ExitCode: 0,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode response", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	slog.Info("update_status command completed successfully")
}
