package docker

import (
	"strings"
	"testing"
)

func TestFormatClaudeOutput_RawText(t *testing.T) {
	rawText := `This is just plain text
Not JSON at all
Should be returned as-is`

	output := FormatClaudeOutput(rawText)

	if output != rawText {
		t.Error("Raw text should be returned unchanged")
	}
}

func TestFormatClaudeOutput_MixedContent(t *testing.T) {
	// If any line fails to parse as JSON, the whole thing should be returned as raw
	mixedContent := `{"type":"system","subtype":"init","session_id":"test"}
This is not JSON
{"type":"assistant","message":{}}`

	output := FormatClaudeOutput(mixedContent)

	if output != mixedContent {
		t.Error("Mixed content should be returned as raw text")
	}
}

func TestFormatClaudeOutput_EmptyInput(t *testing.T) {
	output := FormatClaudeOutput("")

	if output != "" {
		t.Error("Empty input should return empty output")
	}
}

func TestFormatClaudeOutput_ToolInput(t *testing.T) {
	jsonWithTool := `{"type":"system","subtype":"init","session_id":"test","model":"claude-sonnet-4-5","cwd":"/test"}
{"type":"assistant","message":{"model":"claude-sonnet-4-5","id":"msg_123","type":"message","role":"assistant","content":[{"type":"tool_use","id":"tool_1","name":"Read","input":{"file_path":"/test/file.txt"}}],"usage":{"input_tokens":100,"output_tokens":50}}}`

	output := FormatClaudeOutput(jsonWithTool)

	// Should contain tool information in compact format
	if !strings.Contains(output, "> Read") {
		t.Error("Should display tool name")
	}

	if !strings.Contains(output, "/test/file.txt") {
		t.Error("Should display tool input values")
	}
}

func TestFormatClaudeOutput_TokenUsage(t *testing.T) {
	jsonWithUsage := `{"type":"system","subtype":"init","session_id":"test","model":"claude-sonnet-4-5","cwd":"/test"}
{"type":"assistant","message":{"model":"claude-sonnet-4-5","id":"msg_123","type":"message","role":"assistant","content":[{"type":"text","text":"Test"}],"usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":200,"cache_creation_input_tokens":300}}}`

	output := FormatClaudeOutput(jsonWithUsage)

	// Should contain the text content
	if !strings.Contains(output, "Test") {
		t.Error("Should display message text")
	}

	// Compact format doesn't show per-turn token usage anymore
	if !strings.Contains(output, "# Claude Code") {
		t.Error("Should have header")
	}
}

func TestFormatClaudeOutput_CostBreakdown(t *testing.T) {
	jsonWithCost := `{"type":"system","subtype":"init","session_id":"test","model":"claude-sonnet-4-5","cwd":"/test"}
{"type":"result","subtype":"success","total_cost_usd":0.05,"modelUsage":{"claude-sonnet-4-5":{"costUSD":0.03,"inputTokens":100,"outputTokens":50},"claude-haiku-4-5":{"costUSD":0.02,"inputTokens":200,"outputTokens":100}}}`

	output := FormatClaudeOutput(jsonWithCost)

	// Compact format shows cost in summary line
	if !strings.Contains(output, "$0.05") {
		t.Errorf("Should display cost. Output:\n%s", output)
	}

	if !strings.Contains(output, "Success") {
		t.Errorf("Should display success status. Output:\n%s", output)
	}
}

func TestFormatClaudeOutput_PerformanceMetrics(t *testing.T) {
	jsonWithPerf := `{"type":"system","subtype":"init","session_id":"test","model":"claude-sonnet-4-5","cwd":"/test"}
{"type":"result","subtype":"success","duration_ms":5000,"duration_api_ms":4500,"num_turns":3}`

	output := FormatClaudeOutput(jsonWithPerf)

	// Compact format shows metrics in summary line
	if !strings.Contains(output, "5.0s") {
		t.Errorf("Should display duration. Output:\n%s", output)
	}

	if !strings.Contains(output, "3 turns") {
		t.Errorf("Should display number of turns. Output:\n%s", output)
	}

	if !strings.Contains(output, "Success") {
		t.Errorf("Should display success status. Output:\n%s", output)
	}
}
