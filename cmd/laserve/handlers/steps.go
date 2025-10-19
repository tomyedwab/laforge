package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/tomyedwab/laforge/cmd/laserve/websocket"
	"github.com/tomyedwab/laforge/steps"
)

type StepHandler struct {
	db       *steps.StepDatabase
	wsServer *websocket.Server
}

func NewStepHandler(db *steps.StepDatabase, wsServer *websocket.Server) *StepHandler {
	return &StepHandler{db: db, wsServer: wsServer}
}

// StepResponse represents the API response format for steps
type StepResponse struct {
	ID               int               `json:"id"`
	ProjectID        string            `json:"project_id"`
	Active           bool              `json:"active"`
	ParentStepID     *int              `json:"parent_step_id"`
	CommitSHABefore  string            `json:"commit_before"`
	CommitSHAAfter   string            `json:"commit_after"`
	AgentConfig      steps.AgentConfig `json:"agent_config"`
	StartTime        time.Time         `json:"start_time"`
	EndTime          *time.Time        `json:"end_time"`
	DurationMs       *int              `json:"duration_ms"`
	PromptTokens     int               `json:"prompt_tokens"`
	CompletionTokens int               `json:"completion_tokens"`
	TotalTokens      int               `json:"total_tokens"`
	CostUSD          float64           `json:"cost_usd"`
	ExitCode         *int              `json:"exit_code"`
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
		AgentConfig:      step.AgentConfig,
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

	// Get steps from database
	dbSteps, err := h.db.ListSteps(projectID, activeOnly)
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

	step, err := h.db.GetStep(stepID)
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
