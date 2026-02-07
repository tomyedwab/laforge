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
}

// NewHandler creates a new bash proxy handler
func NewHandler(tokenManager *TokenManager, networkName string) *Handler {
	return &Handler{
		tokenManager: tokenManager,
		networkName:  networkName,
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
