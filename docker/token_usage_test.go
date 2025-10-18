package docker

import (
	"testing"
)

func TestExtractTokenUsageFromLogs(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name     string
		logs     string
		expected struct {
			PromptTokens     int
			CompletionTokens int
			TotalTokens      int
			Cost             float64
		}
	}{
		{
			name: "JSON token usage pattern",
			logs: `2025-01-01 12:00:00 Agent started
{"token_usage": {"prompt_tokens": 150, "completion_tokens": 250, "total_tokens": 400, "cost": 0.008}}
2025-01-01 12:00:01 Agent completed`,
			expected: struct {
				PromptTokens     int
				CompletionTokens int
				TotalTokens      int
				Cost             float64
			}{
				PromptTokens:     150,
				CompletionTokens: 250,
				TotalTokens:      400,
				Cost:             0.008,
			},
		},
		{
			name: "TOKEN_USAGE structured log pattern",
			logs: `2025-01-01 12:00:00 Agent started
TOKEN_USAGE: prompt_tokens=150, completion_tokens=250, total_tokens=400, cost=0.008
2025-01-01 12:00:01 Agent completed`,
			expected: struct {
				PromptTokens     int
				CompletionTokens int
				TotalTokens      int
				Cost             float64
			}{
				PromptTokens:     150,
				CompletionTokens: 250,
				TotalTokens:      400,
				Cost:             0.008,
			},
		},
		{
			name: "Resource usage logging pattern",
			logs: `2025-01-01 12:00:00 Agent started
2025-01-01 12:00:01 Resource usage: tokens {"prompt_tokens": 150, "completion_tokens": 250, "total_tokens": 400, "cost": 0.008}
2025-01-01 12:00:02 Agent completed`,
			expected: struct {
				PromptTokens     int
				CompletionTokens int
				TotalTokens      int
				Cost             float64
			}{
				PromptTokens:     150,
				CompletionTokens: 250,
				TotalTokens:      400,
				Cost:             0.008,
			},
		},
		{
			name: "Mixed case TOKEN_USAGE",
			logs: `2025-01-01 12:00:00 Agent started
token_usage: prompt_tokens=100, completion_tokens=200, total_tokens=300, cost=0.006
2025-01-01 12:00:01 Agent completed`,
			expected: struct {
				PromptTokens     int
				CompletionTokens int
				TotalTokens      int
				Cost             float64
			}{
				PromptTokens:     100,
				CompletionTokens: 200,
				TotalTokens:      300,
				Cost:             0.006,
			},
		},
		{
			name: "Partial token usage data",
			logs: `2025-01-01 12:00:00 Agent started
TOKEN_USAGE: prompt_tokens=150, completion_tokens=250
2025-01-01 12:00:01 Agent completed`,
			expected: struct {
				PromptTokens     int
				CompletionTokens int
				TotalTokens      int
				Cost             float64
			}{
				PromptTokens:     150,
				CompletionTokens: 250,
				TotalTokens:      400, // Should be calculated automatically
				Cost:             0,
			},
		},
		{
			name: "No token usage data",
			logs: `2025-01-01 12:00:00 Agent started
2025-01-01 12:00:01 Agent completed
2025-01-01 12:00:02 Cleanup finished`,
			expected: struct {
				PromptTokens     int
				CompletionTokens int
				TotalTokens      int
				Cost             float64
			}{
				PromptTokens:     0,
				CompletionTokens: 0,
				TotalTokens:      0,
				Cost:             0,
			},
		},
		{
			name: "Multiple token usage entries - first one wins",
			logs: `2025-01-01 12:00:00 Agent started
TOKEN_USAGE: prompt_tokens=100, completion_tokens=200, total_tokens=300, cost=0.006
TOKEN_USAGE: prompt_tokens=150, completion_tokens=250, total_tokens=400, cost=0.008
2025-01-01 12:00:01 Agent completed`,
			expected: struct {
				PromptTokens     int
				CompletionTokens int
				TotalTokens      int
				Cost             float64
			}{
				PromptTokens:     100,
				CompletionTokens: 200,
				TotalTokens:      300,
				Cost:             0.006,
			},
		},
		{
			name: "JSON with missing fields",
			logs: `2025-01-01 12:00:00 Agent started
{"token_usage": {"prompt_tokens": 150, "completion_tokens": 250}}
2025-01-01 12:00:01 Agent completed`,
			expected: struct {
				PromptTokens     int
				CompletionTokens int
				TotalTokens      int
				Cost             float64
			}{
				PromptTokens:     150,
				CompletionTokens: 250,
				TotalTokens:      400, // Should be calculated automatically
				Cost:             0,
			},
		},
		{
			name: "Malformed JSON - should be ignored",
			logs: `2025-01-01 12:00:00 Agent started
{"token_usage": {"prompt_tokens": invalid, "completion_tokens": 250, "total_tokens": 400, "cost": 0.008}}
2025-01-01 12:00:01 Agent completed`,
			expected: struct {
				PromptTokens     int
				CompletionTokens int
				TotalTokens      int
				Cost             float64
			}{
				PromptTokens:     0,
				CompletionTokens: 0,
				TotalTokens:      0,
				Cost:             0,
			},
		},
		{
			name: "Invalid token values - should be ignored",
			logs: `2025-01-01 12:00:00 Agent started
TOKEN_USAGE: prompt_tokens=invalid, completion_tokens=abc, total_tokens=xyz, cost=invalid
2025-01-01 12:00:01 Agent completed`,
			expected: struct {
				PromptTokens     int
				CompletionTokens int
				TotalTokens      int
				Cost             float64
			}{
				PromptTokens:     0,
				CompletionTokens: 0,
				TotalTokens:      0,
				Cost:             0,
			},
		},
		{
			name: "Resource usage with extra text",
			logs: `2025-01-01 12:00:00 Agent started
2025-01-01 12:00:01 [INFO] Resource usage: tokens and memory {"prompt_tokens": 75, "completion_tokens": 125, "total_tokens": 200, "cost": 0.004} additional data
2025-01-01 12:00:02 Agent completed`,
			expected: struct {
				PromptTokens     int
				CompletionTokens int
				TotalTokens      int
				Cost             float64
			}{
				PromptTokens:     75,
				CompletionTokens: 125,
				TotalTokens:      200,
				Cost:             0.004,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.extractTokenUsageFromLogs(tt.logs)
			if result.PromptTokens != tt.expected.PromptTokens {
				t.Errorf("PromptTokens = %v, want %v", result.PromptTokens, tt.expected.PromptTokens)
			}
			if result.CompletionTokens != tt.expected.CompletionTokens {
				t.Errorf("CompletionTokens = %v, want %v", result.CompletionTokens, tt.expected.CompletionTokens)
			}
			if result.TotalTokens != tt.expected.TotalTokens {
				t.Errorf("TotalTokens = %v, want %v", result.TotalTokens, tt.expected.TotalTokens)
			}
			if result.Cost != tt.expected.Cost {
				t.Errorf("Cost = %v, want %v", result.Cost, tt.expected.Cost)
			}
		})
	}
}
