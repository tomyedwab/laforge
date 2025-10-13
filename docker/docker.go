package docker

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ContainerConfig represents configuration for creating a container
type ContainerConfig struct {
	Image       string
	Name        string
	WorkDir     string
	TaskDBPath  string
	Environment map[string]string
	Cmd         []string
	MemoryLimit string
	CPUShares   int64
	AutoRemove  bool
	Timeout     time.Duration
}

// Container represents a running Docker container
type Container struct {
	ID        string
	Name      string
	Config    *ContainerConfig
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
func (c *Client) CreateAgentContainer(config *ContainerConfig) (*Container, error) {
	// Validate configuration
	if config.Image == "" {
		return nil, fmt.Errorf("container image is required")
	}
	if config.WorkDir == "" {
		return nil, fmt.Errorf("work directory is required")
	}
	if config.TaskDBPath == "" {
		return nil, fmt.Errorf("task database path is required")
	}

	// Ensure the image exists, pull if necessary
	if err := c.ensureImage(config.Image); err != nil {
		return nil, fmt.Errorf("failed to ensure image %s: %w", config.Image, err)
	}

	// Container name
	containerName := config.Name
	if containerName == "" {
		containerName = fmt.Sprintf("laforge-agent-%d", time.Now().Unix())
	}

	return &Container{
		Name:   containerName,
		Config: config,
	}, nil
}

// StartContainer starts a container
func (c *Client) StartContainer(container *Container) error {
	ctx := context.Background()

	// Build docker run command
	args := []string{"run", "-d", "--name", container.Name}

	// Add environment variables
	args = append(args, "-e", fmt.Sprintf("TASKS_DB_PATH=%s", container.Config.TaskDBPath))
	args = append(args, "-e", "LAFORGE_AGENT=true")

	for key, value := range container.Config.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Add volume mounts
	args = append(args, "-v", fmt.Sprintf("%s:/workspace", container.Config.WorkDir))

	// Mount task database if it's in a different location
	taskDBDir := filepath.Dir(container.Config.TaskDBPath)
	if taskDBDir != container.Config.WorkDir {
		args = append(args, "-v", fmt.Sprintf("%s:/data", taskDBDir))
	}

	// Set working directory
	args = append(args, "-w", "/workspace")

	// Set memory limit if specified
	if container.Config.MemoryLimit != "" {
		args = append(args, "-m", container.Config.MemoryLimit)
	}

	// Set CPU shares if specified
	if container.Config.CPUShares > 0 {
		args = append(args, "-c", fmt.Sprintf("%d", container.Config.CPUShares))
	}

	// Auto-remove if specified
	if container.Config.AutoRemove {
		args = append(args, "--rm")
	}

	// Add image and command
	args = append(args, container.Config.Image)
	if len(container.Config.Cmd) > 0 {
		args = append(args, container.Config.Cmd...)
	}

	// Run the container
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("failed to start container: %w\nOutput: %s", err, string(exitErr.Stderr))
		}
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Extract container ID from output
	container.ID = strings.TrimSpace(string(output))
	container.StartTime = time.Now()

	return nil
}

// WaitForContainer waits for a container to finish
func (c *Client) WaitForContainer(container *Container) (int64, error) {
	ctx := context.Background()

	// Set timeout if specified
	if container.Config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, container.Config.Timeout)
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

	if stdout {
		args = append(args, "--stdout")
	}
	if stderr {
		args = append(args, "--stderr")
	}
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
}

// RunAgentContainer creates, starts, and manages an agent container
func (c *Client) RunAgentContainer(config *ContainerConfig) (int64, string, error) {
	return c.RunAgentContainerWithMetrics(config, nil)
}

// RunAgentContainerWithMetrics creates, starts, and manages an agent container with metrics collection
func (c *Client) RunAgentContainerWithMetrics(config *ContainerConfig, metrics *ContainerMetrics) (int64, string, error) {
	if metrics == nil {
		metrics = &ContainerMetrics{}
	}
	metrics.StartTime = time.Now()

	// Create container
	container, err := c.CreateAgentContainer(config)
	if err != nil {
		metrics.EndTime = time.Now()
		return -1, "", fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	if err := c.StartContainer(container); err != nil {
		// Clean up on error
		metrics.EndTime = time.Now()
		if config.AutoRemove {
			_ = c.CleanupContainer(container)
		}
		return -1, "", fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for container to finish
	exitCode, err := c.WaitForContainer(container)
	if err != nil {
		// Clean up on error
		metrics.EndTime = time.Now()
		metrics.ExitCode = exitCode
		if config.AutoRemove {
			_ = c.CleanupContainer(container)
		}
		return -1, "", fmt.Errorf("failed to wait for container: %w", err)
	}

	metrics.ExitCode = exitCode
	metrics.EndTime = time.Now()

	// Get logs
	logs, err := c.GetContainerLogs(container, true, true, false)
	if err != nil {
		// Clean up on error
		if config.AutoRemove {
			_ = c.CleanupContainer(container)
		}
		return exitCode, "", fmt.Errorf("failed to get container logs: %w", err)
	}

	metrics.LogSize = len(logs)
	metrics.ErrorCount = c.countErrorsInLogs(logs)
	metrics.WarningCount = c.countWarningsInLogs(logs)

	// Clean up if auto-remove is enabled
	if config.AutoRemove {
		if err := c.CleanupContainer(container); err != nil {
			return exitCode, logs, fmt.Errorf("failed to cleanup container: %w", err)
		}
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

// CleanupLaForgeContainers removes all LaForge containers
func (c *Client) CleanupLaForgeContainers() error {
	ctx := context.Background()

	// List all containers
	containers, err := c.ListContainers(true, nil)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	var lastErr error
	for _, container := range containers {
		// Check if this is a LaForge container by name
		if name, ok := container["Name"].(string); ok && strings.HasPrefix(name, "laforge-") {
			if id, ok := container["ID"].(string); ok {
				// Stop and remove the container
				stopCmd := exec.CommandContext(ctx, "docker", "stop", id)
				if err := stopCmd.Run(); err != nil {
					lastErr = fmt.Errorf("failed to stop container %s: %w", id, err)
					continue
				}

				rmCmd := exec.CommandContext(ctx, "docker", "rm", "-f", id)
				if err := rmCmd.Run(); err != nil {
					lastErr = fmt.Errorf("failed to remove container %s: %w", id, err)
				}
			}
		}
	}

	return lastErr
}

// isTempFile checks if a path appears to be a temporary file
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
