package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/tomyedwab/laforge/cmd/laserve/auth"
	"github.com/tomyedwab/laforge/cmd/laserve/websocket"
	"github.com/tomyedwab/laforge/lib/errors"
	"github.com/tomyedwab/laforge/lib/projects"
	"github.com/tomyedwab/laforge/lib/steps"
)

type StepHandler struct {
	wsServer   *websocket.Server
	jwtManager *auth.JWTManager
}

func NewStepHandler(wsServer *websocket.Server, jwtManager *auth.JWTManager) *StepHandler {
	return &StepHandler{wsServer: wsServer, jwtManager: jwtManager}
}

// getProjectStepDB opens the step database for the specified project
func (h *StepHandler) getProjectStepDB(projectID string) (*steps.StepDatabase, error) {
	sdb, err := projects.OpenProjectStepDatabase(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to open project step database: %w", err)
	}
	return sdb, nil
}

// StepResponse represents the API response format for steps
type StepResponse struct {
	ID               int        `json:"id"`
	ProjectID        string     `json:"project_id"`
	Active           bool       `json:"active"`
	ParentStepID     *int       `json:"parent_step_id"`
	CommitSHABefore  string     `json:"commit_before"`
	CommitSHAAfter   string     `json:"commit_after"`
	AgentConfigName  string     `json:"agent_config_name"`
	StartTime        time.Time  `json:"start_time"`
	EndTime          *time.Time `json:"end_time"`
	DurationMs       *int       `json:"duration_ms"`
	PromptTokens     int        `json:"prompt_tokens"`
	CompletionTokens int        `json:"completion_tokens"`
	TotalTokens      int        `json:"total_tokens"`
	CostUSD          float64    `json:"cost_usd"`
	ExitCode         *int       `json:"exit_code"`
}

// convertStep converts a steps.Step to StepResponse
func convertStep(step *steps.Step) *StepResponse {
	return &StepResponse{
		ID:               step.ID,
		ProjectID:        step.ProjectID,
		Active:           step.Active,
		ParentStepID:     step.ParentStepID,
		CommitSHABefore:  step.CommitSHABefore,
		CommitSHAAfter:   step.CommitSHAAfter,
		AgentConfigName:  step.AgentConfigName,
		StartTime:        step.StartTime,
		EndTime:          step.EndTime,
		DurationMs:       step.DurationMs,
		PromptTokens:     step.TokenUsage.PromptTokens,
		CompletionTokens: step.TokenUsage.CompletionTokens,
		TotalTokens:      step.TokenUsage.TotalTokens,
		CostUSD:          step.TokenUsage.Cost,
		ExitCode:         step.ExitCode,
	}
}

// ListSteps handles GET /steps
func (h *StepHandler) ListSteps(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["project_id"]

	// Parse query parameters
	activeOnly := r.URL.Query().Get("active") == "true"

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	// Open project step database
	sdb, err := h.getProjectStepDB(projectID)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to open project step database"}}`, http.StatusInternalServerError)
		return
	}
	defer sdb.Close()

	// Get steps from database
	dbSteps, err := sdb.ListSteps(projectID, activeOnly)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to fetch steps"}}`, http.StatusInternalServerError)
		return
	}

	// Convert to response format
	responseSteps := make([]*StepResponse, len(dbSteps))
	for i, step := range dbSteps {
		responseSteps[i] = convertStep(step)
	}

	// Apply pagination
	total := len(responseSteps)
	start := (page - 1) * limit
	end := start + limit
	if start >= total {
		responseSteps = []*StepResponse{}
	} else if end > total {
		responseSteps = responseSteps[start:]
	} else {
		responseSteps = responseSteps[start:end]
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"steps": responseSteps,
			"pagination": map[string]interface{}{
				"page":  page,
				"limit": limit,
				"total": total,
				"pages": (total + limit - 1) / limit,
			},
		},
		"meta": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetStep handles GET /steps/{step_id}
func (h *StepHandler) GetStep(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	stepIDStr := vars["step_id"]

	stepID, err := strconv.Atoi(stepIDStr)
	if err != nil {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid step ID"}}`, http.StatusBadRequest)
		return
	}

	// Get project ID from URL
	projectID := vars["project_id"]

	// Open project step database
	sdb, err := h.getProjectStepDB(projectID)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to open project step database"}}`, http.StatusInternalServerError)
		return
	}
	defer sdb.Close()

	step, err := sdb.GetStep(stepID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":{"code":"NOT_FOUND","message":"Step not found"}}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to fetch step"}}`, http.StatusInternalServerError)
		}
		return
	}

	responseStep := convertStep(step)

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"step": responseStep,
		},
		"meta": map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

type CreateStepRequest struct {
	ParentStepID    *int   `json:"parent_step_id"`
	CommitSHABefore string `json:"commit_sha_before"`
	AgentConfig     string `json:"agent_config"`
}

// LeaseStep handles POST /steps/lease
func (h *StepHandler) LeaseStep(w http.ResponseWriter, r *http.Request) {
	var req CreateStepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid request body"}}`, http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	// Get project ID from URL
	projectID := vars["project_id"]

	agentConfigs, err := projects.LoadAgentsConfig(projectID)
	if err != nil {
		if errors.IsErrorType(err, errors.ErrNotFound) {
			http.Error(w, `{"error":{"code":"NOT_FOUND","message":"Project agent config not found"}}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to load project agent config"}}`, http.StatusInternalServerError)
		}
		return
	}

	configName := req.AgentConfig
	if configName == "" {
		configName = agentConfigs.Default
	}

	_, ok := agentConfigs.Agents[configName]
	if !ok {
		http.Error(w, `{"error":{"code":"NOT_FOUND","message":"Agent config not found"}}`, http.StatusNotFound)
		return
	}

	newStep := &steps.Step{
		ParentStepID:    req.ParentStepID,
		CommitSHABefore: req.CommitSHABefore,
		AgentConfigName: configName,
		ProjectID:       projectID,
	}

	// Open project step database
	sdb, err := h.getProjectStepDB(projectID)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to open project step database"}}`, http.StatusInternalServerError)
		return
	}
	defer sdb.Close()

	stepId, err := sdb.CreateStep(newStep)
	if err != nil {
		log.Printf("Failed to create step: %v", err) // donotcheckin
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to create step"}}`, http.StatusInternalServerError)
		return
	}

	token, err := h.jwtManager.GenerateToken(nil, &stepId)
	if err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"Failed to generate token"}}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"data":{"token":"%s","step_id":"%d"},"meta":{"timestamp":"%s","version":"1.0.0"}}`,
		token, stepId, time.Now().Format(time.RFC3339))
}
