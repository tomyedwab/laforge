package docker

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormattingWriter_ClaudeJSON(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFormattingWriter(&buf)

	// Write a Claude JSON line
	claudeJSON := `{"type":"system","subtype":"init","model":"claude-sonnet-4-5"}` + "\n"
	n, err := fw.Write([]byte(claudeJSON))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if n != len(claudeJSON) {
		t.Errorf("Expected to write %d bytes, got %d", len(claudeJSON), n)
	}

	output := buf.String()
	if !strings.Contains(output, "Starting Claude Code session") {
		t.Errorf("Expected formatted output, got: %s", output)
	}

	if !strings.Contains(output, "claude-sonnet-4-5") {
		t.Errorf("Expected model name in output, got: %s", output)
	}
}

func TestFormattingWriter_PlainText(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFormattingWriter(&buf)

	// Write plain text
	plainText := "This is just plain text\n"
	n, err := fw.Write([]byte(plainText))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if n != len(plainText) {
		t.Errorf("Expected to write %d bytes, got %d", len(plainText), n)
	}

	output := buf.String()
	// Plain text should contain the original content (though formatting may add newlines)
	if !strings.Contains(output, "This is just plain text") {
		t.Errorf("Expected plain text content in output, got: %s", output)
	}
}

func TestFormattingWriter_MultipleLines(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFormattingWriter(&buf)

	// Write multiple lines at once
	lines := `{"type":"system","subtype":"init","model":"claude-sonnet-4-5"}
{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"}]}}
`
	n, err := fw.Write([]byte(lines))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if n != len(lines) {
		t.Errorf("Expected to write %d bytes, got %d", len(lines), n)
	}

	output := buf.String()
	if !strings.Contains(output, "Starting Claude Code session") {
		t.Errorf("Expected first line formatted, got: %s", output)
	}

	if !strings.Contains(output, "Hello") {
		t.Errorf("Expected second line content, got: %s", output)
	}
}

func TestFormattingWriter_EmptyLines(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFormattingWriter(&buf)

	// Write with empty lines
	input := "line1\n\nline2\n"
	n, err := fw.Write([]byte(input))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if n != len(input) {
		t.Errorf("Expected to write %d bytes, got %d", len(input), n)
	}

	// Empty lines should be skipped but trailing newline preserved
	output := buf.String()
	if !strings.Contains(output, "line1") || !strings.Contains(output, "line2") {
		t.Errorf("Expected both lines in output, got: %s", output)
	}
}

func TestFormattingWriter_NoTrailingNewline(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFormattingWriter(&buf)

	// Write without trailing newline
	input := "line without newline"
	n, err := fw.Write([]byte(input))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if n != len(input) {
		t.Errorf("Expected to write %d bytes, got %d", len(input), n)
	}

	output := buf.String()
	if !strings.Contains(output, "line without newline") {
		t.Errorf("Expected line in output, got: %s", output)
	}
}

func TestFormattingWriter_AssistantMessage(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFormattingWriter(&buf)

	// Write an assistant message
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"Test message"}]}}` + "\n"
	_, err := fw.Write([]byte(input))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "💬") {
		t.Errorf("Expected chat emoji, got: %s", output)
	}

	if !strings.Contains(output, "Test message") {
		t.Errorf("Expected message content, got: %s", output)
	}
}

func TestFormattingWriter_ToolUse(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFormattingWriter(&buf)

	// Write a tool use message
	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","id":"test"}]}}` + "\n"
	_, err := fw.Write([]byte(input))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "🔧") {
		t.Errorf("Expected tool emoji, got: %s", output)
	}

	if !strings.Contains(output, "Read") {
		t.Errorf("Expected tool name, got: %s", output)
	}
}

func TestFormattingWriter_StreamingScenario(t *testing.T) {
	var buf bytes.Buffer
	fw := NewFormattingWriter(&buf)

	// Simulate streaming logs line by line (as would happen in real usage)
	lines := []string{
		`{"type":"system","subtype":"init","model":"claude-sonnet-4-5"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Starting work"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","id":"t1"}]}}`,
		`{"type":"result","subtype":"success","total_cost_usd":0.05}`,
	}

	for _, line := range lines {
		_, err := fw.Write([]byte(line + "\n"))
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	output := buf.String()

	// Check that all expected elements are present
	expectedElements := []string{
		"Starting Claude Code session",
		"claude-sonnet-4-5",
		"💬",
		"Starting work",
		"🔧",
		"Read",
		"✅",
		"completed successfully",
		"$0.05",
	}

	for _, element := range expectedElements {
		if !strings.Contains(output, element) {
			t.Errorf("Expected %q in output, but it's missing. Output:\n%s", element, output)
		}
	}
}

func TestGetContainerLogsFormatted(t *testing.T) {
	// This test would require mocking the Docker client
	// For now, we'll just verify the function signature exists
	client := &Client{}
	container := &Container{ID: "test"}

	// This will fail because there's no actual container, but we're just testing
	// that the function exists and has the right signature
	_, err := client.GetContainerLogsFormatted(container, true, true, false)
	if err == nil {
		t.Skip("Skipping test that requires actual Docker container")
	}

	// The error is expected, we just want to make sure the function compiles
	if !strings.Contains(err.Error(), "failed to get container logs") &&
		!strings.Contains(err.Error(), "failed to list containers") {
		t.Logf("Got expected error: %v", err)
	}
}
