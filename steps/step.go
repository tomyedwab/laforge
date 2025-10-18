package steps

import (
	"encoding/json"
	"time"
)

// Step represents a single step execution in the LaForge system
type Step struct {
	ID              int         `json:"id"`
	Active          bool        `json:"active"`
	ParentStepID    *int        `json:"parent_step_id"`
	CommitSHABefore string      `json:"commit_sha_before"`
	CommitSHAAfter  string      `json:"commit_sha_after"`
	AgentConfig     AgentConfig `json:"agent_config"`
	StartTime       time.Time   `json:"start_time"`
	EndTime         *time.Time  `json:"end_time"`
	DurationMs      *int        `json:"duration_ms"`
	TokenUsage      TokenUsage  `json:"token_usage"`
	ExitCode        *int        `json:"exit_code"`
	ProjectID       string      `json:"project_id"`
	CreatedAt       time.Time   `json:"created_at"`
}

// AgentConfig represents the agent configuration used for a step
type AgentConfig struct {
	Model        string            `json:"model"`
	MaxTokens    int               `json:"max_tokens"`
	Temperature  float64           `json:"temperature"`
	SystemPrompt string            `json:"system_prompt"`
	Tools        []string          `json:"tools"`
	Metadata     map[string]string `json:"metadata"`
}

// TokenUsage represents token usage statistics for a step
type TokenUsage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Cost             float64 `json:"cost"`
}

// StepJSON is used for JSON serialization/deserialization
type StepJSON struct {
	ID              int        `json:"id"`
	Active          bool       `json:"active"`
	ParentStepID    *int       `json:"parent_step_id"`
	CommitSHABefore string     `json:"commit_sha_before"`
	CommitSHAAfter  string     `json:"commit_sha_after"`
	AgentConfigJSON string     `json:"agent_config_json"`
	StartTime       time.Time  `json:"start_time"`
	EndTime         *time.Time `json:"end_time"`
	DurationMs      *int       `json:"duration_ms"`
	TokenUsageJSON  string     `json:"token_usage_json"`
	ExitCode        *int       `json:"exit_code"`
	ProjectID       string     `json:"project_id"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ToJSON converts a Step to its JSON representation
func (s *Step) ToJSON() (*StepJSON, error) {
	agentConfigJSON, err := json.Marshal(s.AgentConfig)
	if err != nil {
		return nil, err
	}

	tokenUsageJSON, err := json.Marshal(s.TokenUsage)
	if err != nil {
		return nil, err
	}

	return &StepJSON{
		ID:              s.ID,
		Active:          s.Active,
		ParentStepID:    s.ParentStepID,
		CommitSHABefore: s.CommitSHABefore,
		CommitSHAAfter:  s.CommitSHAAfter,
		AgentConfigJSON: string(agentConfigJSON),
		StartTime:       s.StartTime,
		EndTime:         s.EndTime,
		DurationMs:      s.DurationMs,
		TokenUsageJSON:  string(tokenUsageJSON),
		ExitCode:        s.ExitCode,
		ProjectID:       s.ProjectID,
		CreatedAt:       s.CreatedAt,
	}, nil
}

// FromJSON converts a StepJSON to a Step
func (s *StepJSON) FromJSON() (*Step, error) {
	var agentConfig AgentConfig
	if err := json.Unmarshal([]byte(s.AgentConfigJSON), &agentConfig); err != nil {
		return nil, err
	}

	var tokenUsage TokenUsage
	if err := json.Unmarshal([]byte(s.TokenUsageJSON), &tokenUsage); err != nil {
		return nil, err
	}

	return &Step{
		ID:              s.ID,
		Active:          s.Active,
		ParentStepID:    s.ParentStepID,
		CommitSHABefore: s.CommitSHABefore,
		CommitSHAAfter:  s.CommitSHAAfter,
		AgentConfig:     agentConfig,
		StartTime:       s.StartTime,
		EndTime:         s.EndTime,
		DurationMs:      s.DurationMs,
		TokenUsage:      tokenUsage,
		ExitCode:        s.ExitCode,
		ProjectID:       s.ProjectID,
		CreatedAt:       s.CreatedAt,
	}, nil
}
