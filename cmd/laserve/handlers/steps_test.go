package handlers

import (
	"testing"
	"time"

	"github.com/tomyedwab/laforge/steps"
)

func TestConvertStep(t *testing.T) {
	now := time.Now()
	duration := 1800000 // 30 minutes in milliseconds
	exitCode := 0

	tests := []struct {
		name     string
		step     *steps.Step
		expected *StepResponse
	}{
		{
			name: "Complete step with token usage",
			step: &steps.Step{
				ID:              1,
				ProjectID:       "test-project",
				Active:          true,
				ParentStepID:    nil,
				CommitSHABefore: "abc123",
				CommitSHAAfter:  "def456",
				AgentConfig: steps.AgentConfig{
					Model:        "gpt-4",
					MaxTokens:    2000,
					Temperature:  0.7,
					SystemPrompt: "You are a helpful assistant",
					Tools:        []string{"latasks", "latools"},
					Metadata:     map[string]string{"env": "test"},
				},
				StartTime:  now,
				EndTime:    &now,
				DurationMs: &duration,
				TokenUsage: steps.TokenUsage{
					PromptTokens:     1500,
					CompletionTokens: 800,
					TotalTokens:      2300,
					Cost:             0.023,
				},
				ExitCode: &exitCode,
			},
			expected: &StepResponse{
				ID:               1,
				ProjectID:        "test-project",
				Active:           true,
				ParentStepID:     nil,
				CommitSHABefore:  "abc123",
				CommitSHAAfter:   "def456",
				PromptTokens:     1500,
				CompletionTokens: 800,
				TotalTokens:      2300,
				CostUSD:          0.023,
			},
		},
		{
			name: "Step with parent and no end time",
			step: &steps.Step{
				ID:              2,
				ProjectID:       "test-project",
				Active:          false,
				ParentStepID:    intPtr(1),
				CommitSHABefore: "def456",
				CommitSHAAfter:  "",
				AgentConfig: steps.AgentConfig{
					Model: "gpt-3.5-turbo",
				},
				StartTime:  now,
				EndTime:    nil,
				DurationMs: nil,
				TokenUsage: steps.TokenUsage{
					PromptTokens:     0,
					CompletionTokens: 0,
					TotalTokens:      0,
					Cost:             0.0,
				},
				ExitCode: nil,
			},
			expected: &StepResponse{
				ID:               2,
				ProjectID:        "test-project",
				Active:           false,
				ParentStepID:     intPtr(1),
				CommitSHABefore:  "def456",
				CommitSHAAfter:   "",
				PromptTokens:     0,
				CompletionTokens: 0,
				TotalTokens:      0,
				CostUSD:          0.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertStep(tt.step)

			if result.ID != tt.expected.ID {
				t.Errorf("Expected ID %d, got %d", tt.expected.ID, result.ID)
			}
			if result.ProjectID != tt.expected.ProjectID {
				t.Errorf("Expected ProjectID %s, got %s", tt.expected.ProjectID, result.ProjectID)
			}
			if result.Active != tt.expected.Active {
				t.Errorf("Expected Active %v, got %v", tt.expected.Active, result.Active)
			}
			if result.ParentStepID != nil && tt.expected.ParentStepID != nil {
				if *result.ParentStepID != *tt.expected.ParentStepID {
					t.Errorf("Expected ParentStepID %v, got %v", *tt.expected.ParentStepID, *result.ParentStepID)
				}
			} else if result.ParentStepID != tt.expected.ParentStepID {
				t.Errorf("Expected ParentStepID %v, got %v", tt.expected.ParentStepID, result.ParentStepID)
			}
			if result.CommitSHABefore != tt.expected.CommitSHABefore {
				t.Errorf("Expected CommitSHABefore %s, got %s", tt.expected.CommitSHABefore, result.CommitSHABefore)
			}
			if result.CommitSHAAfter != tt.expected.CommitSHAAfter {
				t.Errorf("Expected CommitSHAAfter %s, got %s", tt.expected.CommitSHAAfter, result.CommitSHAAfter)
			}
			if result.PromptTokens != tt.expected.PromptTokens {
				t.Errorf("Expected PromptTokens %d, got %d", tt.expected.PromptTokens, result.PromptTokens)
			}
			if result.CompletionTokens != tt.expected.CompletionTokens {
				t.Errorf("Expected CompletionTokens %d, got %d", tt.expected.CompletionTokens, result.CompletionTokens)
			}
			if result.TotalTokens != tt.expected.TotalTokens {
				t.Errorf("Expected TotalTokens %d, got %d", tt.expected.TotalTokens, result.TotalTokens)
			}
			if result.CostUSD != tt.expected.CostUSD {
				t.Errorf("Expected CostUSD %f, got %f", tt.expected.CostUSD, result.CostUSD)
			}
		})
	}
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}

func TestStepResponseStructure(t *testing.T) {
	// Test that the response structure matches the expected API format
	step := &steps.Step{
		ID:              1,
		ProjectID:       "test-project",
		Active:          true,
		ParentStepID:    nil,
		CommitSHABefore: "abc123",
		CommitSHAAfter:  "def456",
		AgentConfig: steps.AgentConfig{
			Model:        "gpt-4",
			MaxTokens:    2000,
			Temperature:  0.7,
			SystemPrompt: "Test prompt",
			Tools:        []string{"tool1", "tool2"},
			Metadata:     map[string]string{"key": "value"},
		},
		StartTime:  time.Now(),
		EndTime:    nil,
		DurationMs: nil,
		TokenUsage: steps.TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Cost:             0.0015,
		},
		ExitCode: nil,
	}

	response := convertStep(step)

	// Verify all expected fields are present
	if response.ID != step.ID {
		t.Error("ID field mismatch")
	}
	if response.ProjectID != step.ProjectID {
		t.Error("ProjectID field mismatch")
	}
	if response.Active != step.Active {
		t.Error("Active field mismatch")
	}
	if response.ParentStepID != step.ParentStepID {
		t.Error("ParentStepID field mismatch")
	}
	if response.CommitSHABefore != step.CommitSHABefore {
		t.Error("CommitSHABefore field mismatch")
	}
	if response.CommitSHAAfter != step.CommitSHAAfter {
		t.Error("CommitSHAAfter field mismatch")
	}
	if response.AgentConfig.Model != step.AgentConfig.Model {
		t.Error("AgentConfig.Model field mismatch")
	}
	if response.StartTime != step.StartTime {
		t.Error("StartTime field mismatch")
	}
	if response.EndTime != step.EndTime {
		t.Error("EndTime field mismatch")
	}
	if response.DurationMs != step.DurationMs {
		t.Error("DurationMs field mismatch")
	}
	if response.PromptTokens != step.TokenUsage.PromptTokens {
		t.Error("PromptTokens field mismatch")
	}
	if response.CompletionTokens != step.TokenUsage.CompletionTokens {
		t.Error("CompletionTokens field mismatch")
	}
	if response.TotalTokens != step.TokenUsage.TotalTokens {
		t.Error("TotalTokens field mismatch")
	}
	if response.CostUSD != step.TokenUsage.Cost {
		t.Error("CostUSD field mismatch")
	}
	if response.ExitCode != step.ExitCode {
		t.Error("ExitCode field mismatch")
	}
}
