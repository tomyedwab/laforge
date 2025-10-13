package logging

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"
)

func TestLogger(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	logger := &Logger{
		level:  DEBUG,
		output: log.New(&buf, "", 0),
	}

	// Test basic logging
	logger.Info("test", "test message")
	output := buf.String()

	if !strings.Contains(output, "test message") {
		t.Errorf("Expected log to contain 'test message', got: %s", output)
	}

	if !strings.Contains(output, "INFO") {
		t.Errorf("Expected log to contain 'INFO', got: %s", output)
	}
}

func TestStepLogger(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	baseLogger := &Logger{
		level:  DEBUG,
		output: log.New(&buf, "", 0),
	}

	stepLogger := NewStepLogger(baseLogger, "test-project", "S123")

	// Test step start logging
	buf.Reset()
	stepLogger.LogStepStart("test-project")
	output := buf.String()

	if !strings.Contains(output, "Starting LaForge step") {
		t.Errorf("Expected log to contain 'Starting LaForge step', got: %s", output)
	}

	if !strings.Contains(output, "test-project") {
		t.Errorf("Expected log to contain project ID, got: %s", output)
	}

	// Test step end logging
	buf.Reset()
	duration := 5 * time.Second
	stepLogger.LogStepEnd(true, duration, 0)
	output = buf.String()

	if !strings.Contains(output, "completed") {
		t.Errorf("Expected log to contain 'completed', got: %s", output)
	}

	if !strings.Contains(output, "5s") {
		t.Errorf("Expected log to contain duration, got: %s", output)
	}
}

func TestLogLevels(t *testing.T) {
	// Test log level string representations
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{FATAL, "FATAL"},
	}

	for _, test := range tests {
		if got := test.level.String(); got != test.expected {
			t.Errorf("Expected %v.String() to be %s, got %s", test.level, test.expected, got)
		}
	}
}

func TestStepTimer(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	logger := &Logger{
		level:  DEBUG,
		output: log.New(&buf, "", 0),
	}

	timer := logger.StartStepTimer("test-step")
	time.Sleep(10 * time.Millisecond) // Small delay to ensure measurable duration
	timer.EndStep("Test operation")

	output := buf.String()
	if !strings.Contains(output, "completed") {
		t.Errorf("Expected log to contain 'completed', got: %s", output)
	}

	if !strings.Contains(output, "Test operation completed") {
		t.Errorf("Expected log to contain 'Test operation completed', got: %s", output)
	}
}
