package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// Logger provides structured logging capabilities
type Logger struct {
	mu         sync.Mutex
	level      LogLevel
	output     *log.Logger
	fileOutput *log.Logger
	stepID     string
	projectID  string
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	StepID    string                 `json:"step_id,omitempty"`
	ProjectID string                 `json:"project_id,omitempty"`
	Component string                 `json:"component,omitempty"`
	Duration  *time.Duration         `json:"duration,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

var (
	// Global logger instance
	globalLogger *Logger
	initOnce     sync.Once
)

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	initOnce.Do(func() {
		globalLogger = NewLogger(INFO, "")
	})
	return globalLogger
}

// NewLogger creates a new logger instance
func NewLogger(level LogLevel, logFile string) *Logger {
	logger := &Logger{
		level:  level,
		output: log.New(os.Stdout, "", 0),
	}

	// Set up file logging if specified
	if logFile != "" {
		if fileLogger, err := setupFileLogger(logFile); err == nil {
			logger.fileOutput = fileLogger
		} else {
			logger.output.Printf("Failed to setup file logging: %v", err)
		}
	}

	return logger
}

// setupFileLogger creates a file-based logger
func setupFileLogger(logFile string) (*log.Logger, error) {
	// Ensure directory exists
	dir := filepath.Dir(logFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file in append mode
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return log.New(file, "", 0), nil
}

// SetStepID sets the step ID for contextual logging
func (l *Logger) SetStepID(stepID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stepID = stepID
}

// SetProjectID sets the project ID for contextual logging
func (l *Logger) SetProjectID(projectID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.projectID = projectID
}

// log formats and outputs a log message
func (l *Logger) log(level LogLevel, component, message string, metadata map[string]interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Get caller information
	_, file, line, ok := runtime.Caller(2)
	if ok {
		file = filepath.Base(file)
	}

	error := ""
	if err, ok := metadata["error"]; ok {
		error = fmt.Sprintf("%v", err)
	}

	// Create log entry
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level.String(),
		Message:   message,
		StepID:    l.stepID,
		ProjectID: l.projectID,
		Component: component,
		Metadata:  metadata,
		Error:     error,
	}

	// Format log message
	logMessage := l.formatLogMessage(entry, file, line)

	// Output to console
	l.output.Println(logMessage)

	// Output to file if configured
	if l.fileOutput != nil {
		l.fileOutput.Println(logMessage)
	}
}

// formatLogMessage formats a log entry into a string
func (l *Logger) formatLogMessage(entry LogEntry, file string, line int) string {
	var sb strings.Builder

	// Timestamp
	sb.WriteString(entry.Timestamp.Format("2006-01-02 15:04:05.000"))
	sb.WriteString(" ")

	// Level with color coding
	level := LogLevel(0) // Default to DEBUG
	switch entry.Level {
	case "DEBUG":
		level = DEBUG
	case "INFO":
		level = INFO
	case "WARN":
		level = WARN
	case "ERROR":
		level = ERROR
	case "FATAL":
		level = FATAL
	}
	sb.WriteString(level.Color())
	sb.WriteString(fmt.Sprintf("%-5s", entry.Level))
	sb.WriteString("\033[0m") // Reset color
	sb.WriteString(" ")

	// Context information
	if entry.ProjectID != "" {
		sb.WriteString(fmt.Sprintf("[%s]", entry.ProjectID))
	}
	if entry.StepID != "" {
		sb.WriteString(fmt.Sprintf("[%s]", entry.StepID))
	}
	if entry.Component != "" {
		sb.WriteString(fmt.Sprintf("[%s]", entry.Component))
	}
	if entry.ProjectID != "" || entry.StepID != "" || entry.Component != "" {
		sb.WriteString(" ")
	}

	// Message
	sb.WriteString(entry.Message)

	// Source location
	if file != "" && line > 0 {
		sb.WriteString(fmt.Sprintf(" (%s:%d)", file, line))
	}

	// Duration
	if entry.Duration != nil {
		sb.WriteString(fmt.Sprintf(" [duration: %v]", *entry.Duration))
	}

	// Error
	if entry.Error != "" {
		sb.WriteString(fmt.Sprintf(" [error: %s]", entry.Error))
	}

	return sb.String()
}

// Debug logs a debug message
func (l *Logger) Debug(component, message string, metadata ...map[string]interface{}) {
	meta := mergeMetadata(metadata...)
	l.log(DEBUG, component, message, meta)
}

// Info logs an info message
func (l *Logger) Info(component, message string, metadata ...map[string]interface{}) {
	meta := mergeMetadata(metadata...)
	l.log(INFO, component, message, meta)
}

// Warn logs a warning message
func (l *Logger) Warn(component, message string, metadata ...map[string]interface{}) {
	meta := mergeMetadata(metadata...)
	l.log(WARN, component, message, meta)
}

// Error logs an error message
func (l *Logger) Error(component, message string, err error, metadata ...map[string]interface{}) {
	meta := mergeMetadata(metadata...)
	if err != nil {
		if meta == nil {
			meta = make(map[string]interface{})
		}
		meta["error"] = err.Error()
	}
	l.log(ERROR, component, message, meta)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(component, message string, err error, metadata ...map[string]interface{}) {
	meta := mergeMetadata(metadata...)
	if err != nil {
		if meta == nil {
			meta = make(map[string]interface{})
		}
		meta["error"] = err.Error()
	}
	l.log(FATAL, component, message, meta)
	os.Exit(1)
}

// StepTimer provides timing functionality for steps
type StepTimer struct {
	logger    *Logger
	stepName  string
	startTime time.Time
}

// StartStepTimer starts timing a step
func (l *Logger) StartStepTimer(stepName string) *StepTimer {
	return &StepTimer{
		logger:    l,
		stepName:  stepName,
		startTime: time.Now(),
	}
}

// EndStep ends the step timing and logs the duration
func (st *StepTimer) EndStep(message string) {
	duration := time.Since(st.startTime)
	metadata := map[string]interface{}{
		"step_name": st.stepName,
		"duration":  duration.String(),
	}
	st.logger.Info("step", fmt.Sprintf("%s completed in %v", message, duration), metadata)
}

// mergeMetadata merges multiple metadata maps
func mergeMetadata(maps ...map[string]interface{}) map[string]interface{} {
	if len(maps) == 0 {
		return nil
	}
	if len(maps) == 1 {
		return maps[0]
	}

	result := make(map[string]interface{})
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// String returns the string representation of a log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Color returns the ANSI color code for a log level
func (l LogLevel) Color() string {
	switch l {
	case DEBUG:
		return "\033[36m" // Cyan
	case INFO:
		return "\033[32m" // Green
	case WARN:
		return "\033[33m" // Yellow
	case ERROR:
		return "\033[31m" // Red
	case FATAL:
		return "\033[35m" // Magenta
	default:
		return "\033[0m" // Reset
	}
}

// Convenience functions for global logger
func Debug(component, message string, metadata ...map[string]interface{}) {
	GetLogger().Debug(component, message, metadata...)
}

func Info(component, message string, metadata ...map[string]interface{}) {
	GetLogger().Info(component, message, metadata...)
}

func Warn(component, message string, metadata ...map[string]interface{}) {
	GetLogger().Warn(component, message, metadata...)
}

func Error(component, message string, err error, metadata ...map[string]interface{}) {
	GetLogger().Error(component, message, err, metadata...)
}

func Fatal(component, message string, err error, metadata ...map[string]interface{}) {
	GetLogger().Fatal(component, message, err, metadata...)
}
