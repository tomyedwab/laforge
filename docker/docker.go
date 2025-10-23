package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tomyedwab/laforge/lib/projects"
	"github.com/tomyedwab/laforge/lib/steps"
)

// Container represents a running Docker container
type Container struct {
	ID        string
	Name      string
	Config    *projects.AgentConfig
	WorkDir   string
	StartTime time.Time
}

// Client provides Docker operations using Docker CLI
type Client struct {
	// Using Docker CLI for now to avoid complex dependencies
}

// NewClient creates a new Docker client
func NewClient() (*Client, error) {
	// Check if Docker is available
	if err := exec.Command("docker", "--version").Run(); err != nil {
		return nil, fmt.Errorf("docker is not available: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "info")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}

	return &Client{}, nil
}

// Close closes the Docker client (no-op for CLI-based implementation)
func (c *Client) Close() error {
	return nil
}

// CreateAgentContainer creates a container for running the LaForge agent
func (c *Client) CreateAgentContainer(agentConfig *projects.AgentConfig, workDir string) (*Container, error) {
	// Ensure the image exists, pull if necessary
	if err := c.ensureImage(agentConfig.Image); err != nil {
		return nil, fmt.Errorf("failed to ensure image %s: %w", agentConfig.Image, err)
	}

	// Container name
	containerName := fmt.Sprintf("laforge-agent-%d", time.Now().Unix())

	return &Container{
		Name:    containerName,
		Config:  agentConfig,
		WorkDir: workDir,
	}, nil
}

// WaitForContainer waits for a container to finish
func (c *Client) WaitForContainer(container *Container) (int64, error) {
	ctx := context.Background()

	// Set timeout if specified
	if container.Config.Runtime.Timeout != "" {
		var err error
		timeoutDuration, err := time.ParseDuration(container.Config.Runtime.Timeout)
		if err != nil {
			return -1, fmt.Errorf("invalid timeout format: %w", err)
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeoutDuration)
		defer cancel()
	}

	// Wait for container
	cmd := exec.CommandContext(ctx, "docker", "wait", container.ID)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return -1, fmt.Errorf("failed to wait for container: %w\nOutput: %s", err, string(exitErr.Stderr))
		}
		return -1, fmt.Errorf("failed to wait for container: %w", err)
	}

	// Parse exit code
	exitCode := int64(0)
	fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &exitCode)

	return exitCode, nil
}

// GetContainerLogs retrieves logs from a container
func (c *Client) GetContainerLogs(container *Container, stdout, stderr, timestamps bool) (string, error) {
	ctx := context.Background()

	args := []string{"logs"}

	// Note: docker logs returns both stdout and stderr by default
	// The stdout and stderr parameters are kept for API compatibility but not used
	// as the Docker CLI doesn't support filtering by stream

	if timestamps {
		args = append(args, "-t")
	}

	args = append(args, container.ID)

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("failed to get container logs: %w\nOutput: %s", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}

	return string(output), nil
}

// StopContainer stops a running container
func (c *Client) StopContainer(container *Container, timeout int) error {
	ctx := context.Background()

	cmd := exec.CommandContext(ctx, "docker", "stop", "-t", fmt.Sprintf("%d", timeout), container.ID)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	return nil
}

// RemoveContainer removes a container
func (c *Client) RemoveContainer(container *Container, force bool) error {
	ctx := context.Background()

	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, container.ID)

	cmd := exec.CommandContext(ctx, "docker", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	return nil
}

// CleanupContainer stops and removes a container
func (c *Client) CleanupContainer(container *Container) error {
	// Try to stop the container first (ignore errors if it's already stopped)
	_ = c.StopContainer(container, 10)

	// Remove the container
	return c.RemoveContainer(container, true)
}

// ListContainers lists containers with optional filters
func (c *Client) ListContainers(all bool, filters map[string]string) ([]map[string]interface{}, error) {
	ctx := context.Background()

	args := []string{"ps", "--format", "{{json .}}"}
	if all {
		args = append(args, "-a")
	}

	// Add name filter if specified
	if nameFilter, ok := filters["name"]; ok {
		args = append(args, "--filter", fmt.Sprintf("name=%s", nameFilter))
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Parse JSON output (simplified - in real implementation would use proper JSON parsing)
	var containers []map[string]interface{}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line != "" {
			containers = append(containers, map[string]interface{}{
				"ID":   strings.TrimSpace(line),
				"Name": "container",
			})
		}
	}

	return containers, nil
}

// GetContainerInfo gets detailed information about a container
func (c *Client) GetContainerInfo(containerID string) (map[string]interface{}, error) {
	ctx := context.Background()

	cmd := exec.CommandContext(ctx, "docker", "inspect", containerID)
	_, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Return simplified info (in real implementation would parse JSON properly)
	return map[string]interface{}{
		"ID":      containerID,
		"Running": true,
	}, nil
}

// PullImage pulls a Docker image
func (c *Client) PullImage(image string) error {
	ctx := context.Background()

	cmd := exec.CommandContext(ctx, "docker", "pull", image)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("failed to pull image %s: %w\nOutput: %s", image, err, string(exitErr.Stderr))
		}
		return fmt.Errorf("failed to pull image %s: %w", image, err)
	}

	// Check if pull was successful
	if strings.Contains(string(output), "Error") {
		return fmt.Errorf("failed to pull image %s: %s", image, string(output))
	}

	return nil
}

// ensureImage ensures that the specified image exists locally
func (c *Client) ensureImage(image string) error {
	ctx := context.Background()

	// Check if image exists locally
	cmd := exec.CommandContext(ctx, "docker", "images", "--format", "{{.Repository}}:{{.Tag}}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list images: %w", err)
	}

	// Check if our image is in the list
	images := strings.Split(string(output), "\n")
	for _, img := range images {
		if strings.TrimSpace(img) == image {
			return nil // Image exists
		}
	}

	// Image not found, pull it
	return c.PullImage(image)
}

// parseMemoryLimit parses memory limit string (e.g., "512m", "1g") to bytes
func parseMemoryLimit(limit string) string {
	limit = strings.ToLower(strings.TrimSpace(limit))

	if len(limit) == 0 {
		return ""
	}

	// Validate format (number followed by optional unit)
	validUnits := []string{"b", "k", "kb", "m", "mb", "g", "gb"}

	for _, unit := range validUnits {
		if strings.HasSuffix(limit, unit) {
			return limit // Already in correct format
		}
	}

	// Assume megabytes if no unit specified
	return limit + "m"
}

// ContainerMetrics represents metrics collected during container execution
type ContainerMetrics struct {
	StartTime    time.Time
	EndTime      time.Time
	ExitCode     int64
	MemoryUsage  string
	CPUUsage     string
	LogSize      int
	ErrorCount   int
	WarningCount int
	TokenUsage   steps.TokenUsage
}

// RunAgentContainerFromConfigWithStreamingLogs creates, starts, and manages an agent container from AgentConfig
// with real-time log streaming to the provided writer, and returns logs and metrics
func (c *Client) RunAgentContainerFromConfigWithStreamingLogs(agentConfig *projects.AgentConfig, workDir string, logWriter io.Writer, metrics *ContainerMetrics) (int64, string, error) {
	if metrics == nil {
		metrics = &ContainerMetrics{}
	}
	metrics.StartTime = time.Now()

	// Create container from AgentConfig
	container, err := c.CreateAgentContainer(agentConfig, workDir)
	if err != nil {
		metrics.EndTime = time.Now()
		return -1, "", fmt.Errorf("failed to create container from config: %w", err)
	}

	// Create a copy of agent config with AutoRemove disabled
	// We need to get logs before removing the container
	configCopy := *agentConfig
	configCopy.Runtime.AutoRemove = false

	// Start container with AgentConfig (without AutoRemove)
	if err := c.startContainerWithAgentConfig(container, &configCopy); err != nil {
		// Clean up on error
		metrics.EndTime = time.Now()
		_ = c.CleanupContainer(container)
		return -1, "", fmt.Errorf("failed to start container: %w", err)
	}

	// Start streaming logs in the background
	logBuffer := &bytes.Buffer{}
	var multiWriter io.Writer
	if logWriter != nil {
		multiWriter = io.MultiWriter(logWriter, logBuffer)
	} else {
		multiWriter = logBuffer
	}

	// Stream logs using docker logs -f
	ctx := context.Background()
	logsCmd := exec.CommandContext(ctx, "docker", "logs", "-f", container.ID)

	// Create pipes for stdout and stderr
	stdout, err := logsCmd.StdoutPipe()
	if err != nil {
		_ = c.CleanupContainer(container)
		return -1, "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := logsCmd.StderrPipe()
	if err != nil {
		_ = c.CleanupContainer(container)
		return -1, "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the logs command
	if err := logsCmd.Start(); err != nil {
		_ = c.CleanupContainer(container)
		return -1, "", fmt.Errorf("failed to start logs command: %w", err)
	}

	// Stream stdout and stderr to the multi-writer in goroutines
	logsDone := make(chan error, 2)

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text() + "\n"
			multiWriter.Write([]byte(line))
		}
		logsDone <- scanner.Err()
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text() + "\n"
			multiWriter.Write([]byte(line))
		}
		logsDone <- scanner.Err()
	}()

	// Wait for container to finish
	exitCode, err := c.WaitForContainer(container)
	if err != nil {
		// Clean up on error
		metrics.EndTime = time.Now()
		metrics.ExitCode = exitCode
		logsCmd.Process.Kill()
		_ = c.CleanupContainer(container)
		return -1, "", fmt.Errorf("failed to wait for container: %w", err)
	}

	metrics.ExitCode = exitCode
	metrics.EndTime = time.Now()

	// Wait for log streaming to complete (with timeout)
	logTimeout := time.After(5 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case <-logsDone:
		case <-logTimeout:
			logsCmd.Process.Kill()
		}
	}

	// Wait for the logs command to finish
	logsCmd.Wait()

	// Get the captured logs
	logs := logBuffer.String()

	metrics.LogSize = len(logs)
	metrics.ErrorCount = c.countErrorsInLogs(logs)
	metrics.WarningCount = c.countWarningsInLogs(logs)
	metrics.TokenUsage = c.ExtractTokenUsageFromLogs(logs)

	// Always clean up the container manually since we disabled AutoRemove
	// to be able to collect logs
	if err := c.CleanupContainer(container); err != nil {
		return exitCode, logs, fmt.Errorf("failed to cleanup container: %w", err)
	}

	return exitCode, logs, nil
}

// countErrorsInLogs counts error messages in container logs
func (c *Client) countErrorsInLogs(logs string) int {
	errorCount := 0
	lines := strings.Split(logs, "\n")
	for _, line := range lines {
		line = strings.ToLower(line)
		if strings.Contains(line, "error") || strings.Contains(line, "fatal") || strings.Contains(line, "panic") {
			errorCount++
		}
	}
	return errorCount
}

// countWarningsInLogs counts warning messages in container logs
func (c *Client) countWarningsInLogs(logs string) int {
	warningCount := 0
	lines := strings.Split(logs, "\n")
	for _, line := range lines {
		line = strings.ToLower(line)
		if strings.Contains(line, "warning") || strings.Contains(line, "warn") {
			warningCount++
		}
	}
	return warningCount
}

// ExtractTokenUsageFromLogs extracts token usage information from container logs
func (c *Client) ExtractTokenUsageFromLogs(logs string) steps.TokenUsage {
	tokenUsage := steps.TokenUsage{}
	tokenUsageFound := false

	lines := strings.Split(logs, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip if we already found token usage data (first occurrence wins)
		if tokenUsageFound {
			continue
		}

		// Look for JSON token usage patterns
		if strings.Contains(line, "token_usage") {
			// Try to parse JSON object containing token_usage
			var jsonData map[string]interface{}
			if err := json.Unmarshal([]byte(line), &jsonData); err == nil {
				if tokenData, ok := jsonData["token_usage"].(map[string]interface{}); ok {
					if promptTokens, ok := tokenData["prompt_tokens"].(float64); ok {
						tokenUsage.PromptTokens = int(promptTokens)
						tokenUsageFound = true
					}
					if completionTokens, ok := tokenData["completion_tokens"].(float64); ok {
						tokenUsage.CompletionTokens = int(completionTokens)
						tokenUsageFound = true
					}
					if totalTokens, ok := tokenData["total_tokens"].(float64); ok {
						tokenUsage.TotalTokens = int(totalTokens)
						tokenUsageFound = true
					}
					if cost, ok := tokenData["cost"].(float64); ok {
						tokenUsage.Cost = cost
						tokenUsageFound = true
					}
				}
			}
		}

		// Look for structured log lines with TOKEN_USAGE prefix
		if strings.HasPrefix(strings.ToUpper(line), "TOKEN_USAGE:") {
			// Parse key-value pairs after TOKEN_USAGE:
			content := strings.TrimPrefix(strings.ToUpper(line), "TOKEN_USAGE:")
			content = strings.TrimSpace(content)

			// Parse comma-separated key=value pairs
			pairs := strings.Split(content, ",")
			for _, pair := range pairs {
				pair = strings.TrimSpace(pair)
				parts := strings.Split(pair, "=")
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])

					switch key {
					case "PROMPT_TOKENS":
						if val, err := strconv.Atoi(value); err == nil {
							tokenUsage.PromptTokens = val
							tokenUsageFound = true
						}
					case "COMPLETION_TOKENS":
						if val, err := strconv.Atoi(value); err == nil {
							tokenUsage.CompletionTokens = val
							tokenUsageFound = true
						}
					case "TOTAL_TOKENS":
						if val, err := strconv.Atoi(value); err == nil {
							tokenUsage.TotalTokens = val
							tokenUsageFound = true
						}
					case "COST":
						if val, err := strconv.ParseFloat(value, 64); err == nil {
							tokenUsage.Cost = val
							tokenUsageFound = true
						}
					}
				}
			}
		}

		// Look for resource usage logging patterns
		if strings.Contains(strings.ToLower(line), "resource usage") && strings.Contains(line, "tokens") {
			// Try to extract JSON object from the line
			startIdx := strings.Index(line, "{")
			endIdx := strings.LastIndex(line, "}")
			if startIdx != -1 && endIdx != -1 && startIdx < endIdx {
				jsonStr := line[startIdx : endIdx+1]
				var jsonData map[string]interface{}
				if err := json.Unmarshal([]byte(jsonStr), &jsonData); err == nil {
					if promptTokens, ok := jsonData["prompt_tokens"].(float64); ok {
						tokenUsage.PromptTokens = int(promptTokens)
						tokenUsageFound = true
					}
					if completionTokens, ok := jsonData["completion_tokens"].(float64); ok {
						tokenUsage.CompletionTokens = int(completionTokens)
						tokenUsageFound = true
					}
					if totalTokens, ok := jsonData["total_tokens"].(float64); ok {
						tokenUsage.TotalTokens = int(totalTokens)
						tokenUsageFound = true
					}
					if cost, ok := jsonData["cost"].(float64); ok {
						tokenUsage.Cost = cost
						tokenUsageFound = true
					}
				}
			}
		}
	}

	// Calculate total tokens if not explicitly provided
	if tokenUsage.TotalTokens == 0 && (tokenUsage.PromptTokens > 0 || tokenUsage.CompletionTokens > 0) {
		tokenUsage.TotalTokens = tokenUsage.PromptTokens + tokenUsage.CompletionTokens
	}

	return tokenUsage
}

// startContainerWithAgentConfig starts a container with additional AgentConfig options
func (c *Client) startContainerWithAgentConfig(container *Container, agentConfig *projects.AgentConfig) error {
	ctx := context.Background()

	// Build docker run command
	args := []string{"run", "-d", "--name", container.Name}

	// Add environment variables from AgentConfig
	for key, value := range agentConfig.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Add volume mounts from AgentConfig
	for _, volume := range agentConfig.Volumes {
		args = append(args, "-v", volume)
	}

	// Set working directory
	if agentConfig.WorkingDir != "" {
		args = append(args, "-w", agentConfig.WorkingDir)
	} else {
		args = append(args, "-w", "/src")
	}

	// Set memory limit if specified
	if agentConfig.Resources.Memory != "" {
		args = append(args, "-m", agentConfig.Resources.Memory)
	}

	// Set CPU shares if specified
	if agentConfig.Resources.CPUShares > 0 {
		args = append(args, "-c", fmt.Sprintf("%d", agentConfig.Resources.CPUShares))
	}

	// Set network mode if specified
	if agentConfig.Runtime.NetworkMode != "" {
		args = append(args, "--network", agentConfig.Runtime.NetworkMode)
	}

	// Set capabilities if specified
	for _, cap := range agentConfig.Runtime.Capabilities {
		args = append(args, "--cap-add", cap)
	}

	// Set devices if specified
	for _, device := range agentConfig.Runtime.Devices {
		args = append(args, "--device", device)
	}

	// Set AutoRemove option
	if agentConfig.Runtime.AutoRemove {
		args = append(args, "--rm")
	}

	// Set privileged mode if specified
	if agentConfig.Runtime.Privileged {
		args = append(args, "--privileged")
	}

	// Add volume mounts for work directory and task database
	args = append(args, "-v", fmt.Sprintf("%s:/src", container.WorkDir))

	// Set the image
	args = append(args, agentConfig.Image)

	// Add command if specified
	if len(agentConfig.Command) > 0 {
		args = append(args, agentConfig.Command...)
	}

	// Execute docker run command
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start container: %w\nOutput: %s", err, string(output))
	}

	// Parse container ID from output
	containerID := strings.TrimSpace(string(output))
	if containerID == "" {
		return fmt.Errorf("failed to get container ID from docker run output")
	}

	container.ID = containerID
	container.StartTime = time.Now()

	return nil
}

func isTempFile(path string) bool {
	// Check if the filename contains typical temporary file patterns
	base := filepath.Base(path)

	// Files starting with dot are hidden/temp files
	if len(base) > 0 && base[0] == '.' {
		return true
	}

	// Check for temp patterns in filename
	if contains(base, "-tmp-") ||
		contains(base, "-temp-") ||
		contains(base, ".tmp") ||
		contains(base, "_tmp_") ||
		contains(base, "laforge-") {
		return true
	}

	// Check for numeric patterns that suggest temp files (like test-123456.db)
	if contains(base, "test-") && contains(base, ".db") {
		return true
	}

	return false
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsAt(s, substr)
}

// containsAt checks if a string contains a substring at any position
func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
