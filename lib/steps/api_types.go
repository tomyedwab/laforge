package steps

import "time"

type LeaseStepRequest struct {
	CommitSHABefore string `json:"commit_sha_before"`
	AgentConfigName string `json:"agent_config_name"`
}

type FinalizeStepRequest struct {
	StepID         int    `json:"step_id"`
	CommitSHAAfter string `json:"commit_sha_after"`
	ExitCode       int    `json:"exit_code"`
	// TODO: Capture token usage
}

type MetaResponse struct {
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

type LeaseStepResponse struct {
	StepID int          `json:"step_id"`
	Token  string       `json:"token"`
	Meta   MetaResponse `json:"meta"`
}
