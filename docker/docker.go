package docker

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tomyedwab/laforge/projects"
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

// convertAgentConfigToContainerConfig converts AgentConfig to ContainerConfig
func (c *Client) convertAgentConfigToContainerConfig(agentConfig *projects.AgentConfig, workDir, taskDBPath string) (*ContainerConfig, error) {
	if agentConfig == nil {
		return nil, fmt.Errorf("agent configuration cannot be nil")
	}

	// Parse timeout duration
	timeoutDuration := time.Duration(0)
	if agentConfig.Runtime.Timeout != "" {
		var err error
		timeoutDuration, err = time.ParseDuration(agentConfig.Runtime.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout format: %w", err)
		}
	}

	// Build environment variables (merge default and custom)
	env := make(map[string]string)
	for k, v := range agentConfig.Environment {
		env[k] = v
	}
	// Add required environment variables
	env["TASKS_DB_PATH"] = taskDBPath
	env["LAFORGE_AGENT"] = "true"

	// Build command
	var cmd []string
	if len(agentConfig.Command) > 0 {
		cmd = agentConfig.Command
	}

	containerConfig := &ContainerConfig{
		Image:       agentConfig.Image,
		Name:        agentConfig.Name,
		WorkDir:     workDir,
		TaskDBPath:  taskDBPath,
		Environment: env,
		Cmd:         cmd,
		MemoryLimit: agentConfig.Resources.Memory,
		CPUShares:   agentConfig.Resources.CPUShares,
		AutoRemove:  agentConfig.Runtime.AutoRemove,
		Timeout:     timeoutDuration,
	}

	return containerConfig, nil
}

// CreateAgentContainerFromConfig creates a container from AgentConfig
func (c *Client) CreateAgentContainerFromConfig(agentConfig *projects.AgentConfig, workDir, taskDBPath string) (*Container, error) {
	if agentConfig == nil {
		return nil, fmt.Errorf("agent configuration cannot be nil")
	}

	// Convert AgentConfig to ContainerConfig
	containerConfig, err := c.convertAgentConfigToContainerConfig(agentConfig, workDir, taskDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to convert agent config: %w", err)
	}

	return c.CreateAgentContainer(containerConfig)
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

// RunAgentContainerFromConfig creates, starts, and manages an agent container from AgentConfig with metrics collection
func (c *Client) RunAgentContainerFromConfig(agentConfig *projects.AgentConfig, workDir, taskDBPath string, metrics *ContainerMetrics) (int64, string, error) {
	if metrics == nil {
		metrics = &ContainerMetrics{}
	}
	metrics.StartTime = time.Now()

	// Create container from AgentConfig
	container, err := c.CreateAgentContainerFromConfig(agentConfig, workDir, taskDBPath)
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

	// Wait for container to finish
	exitCode, err := c.WaitForContainer(container)
	if err != nil {
		// Clean up on error
		metrics.EndTime = time.Now()
		metrics.ExitCode = exitCode
		_ = c.CleanupContainer(container)
		return -1, "", fmt.Errorf("failed to wait for container: %w", err)
	}

	metrics.ExitCode = exitCode
	metrics.EndTime = time.Now()

	// Get logs (container still exists because we disabled AutoRemove)
	logs, err := c.GetContainerLogs(container, true, true, false)
	if err != nil {
		// Clean up on error
		_ = c.CleanupContainer(container)
		return exitCode, "", fmt.Errorf("failed to get container logs: %w", err)
	}

	metrics.LogSize = len(logs)
	metrics.ErrorCount = c.countErrorsInLogs(logs)
	metrics.WarningCount = c.countWarningsInLogs(logs)

	// Always clean up the container manually since we disabled AutoRemove
	// to be able to collect logs
	if err := c.CleanupContainer(container); err != nil {
		return exitCode, logs, fmt.Errorf("failed to cleanup container: %w", err)
	}

	return exitCode, logs, nil
}

// RunAgentContainerFromConfigWithStreamingLogs creates, starts, and manages an agent container from AgentConfig
// with real-time log streaming to the provided writer, and returns logs and metrics
func (c *Client) RunAgentContainerFromConfigWithStreamingLogs(agentConfig *projects.AgentConfig, workDir, taskDBPath string, logWriter io.Writer, metrics *ContainerMetrics) (int64, string, error) {
	if metrics == nil {
		metrics = &ContainerMetrics{}
	}
	metrics.StartTime = time.Now()

	// Create container from AgentConfig
	container, err := c.CreateAgentContainerFromConfig(agentConfig, workDir, taskDBPath)
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

// StartContainerFromConfig starts a container from AgentConfig
func (c *Client) StartContainerFromConfig(agentConfig *projects.AgentConfig, workDir, taskDBPath string) (*Container, error) {
	// Create container from AgentConfig
	container, err := c.CreateAgentContainerFromConfig(agentConfig, workDir, taskDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create container from config: %w", err)
	}

	// Start the container with additional configuration from AgentConfig
	if err := c.startContainerWithAgentConfig(container, agentConfig); err != nil {
		return nil, fmt.Errorf("failed to start container with agent config: %w", err)
	}

	return container, nil
}

// startContainerWithAgentConfig starts a container with additional AgentConfig options
func (c *Client) startContainerWithAgentConfig(container *Container, agentConfig *projects.AgentConfig) error {
	ctx := context.Background()

	// Build docker run command
	args := []string{"run", "-d", "--name", container.Name}

	// Add environment variables
	args = append(args, "-e", fmt.Sprintf("TASKS_DB_PATH=%s", container.Config.TaskDBPath))
	args = append(args, "-e", "LAFORGE_AGENT=true")

	for key, value := range agentConfig.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Add volume mounts from AgentConfig
	for _, volume := range agentConfig.Volumes {
		args = append(args, "-v", volume)
	}

	// Add main volume mounts if not already specified
	hasWorkspaceVolume := false
	hasDataVolume := false
	for _, volume := range agentConfig.Volumes {
		if strings.HasSuffix(volume, ":/src") {
			hasWorkspaceVolume = true
		}
		if strings.HasSuffix(volume, ":/state") {
			hasDataVolume = true
		}
	}

	if !hasWorkspaceVolume {
		args = append(args, "-v", fmt.Sprintf("%s:/src", container.Config.WorkDir))
	}

	// Mount task database if it's in a different location
	taskDBDir := filepath.Dir(container.Config.TaskDBPath)
	if taskDBDir != container.Config.WorkDir && !hasDataVolume {
		args = append(args, "-v", fmt.Sprintf("%s:/state", taskDBDir))
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

	// Set CPU limit if specified
	if agentConfig.Resources.CPULimit != "" {
		args = append(args, "--cpus", agentConfig.Resources.CPULimit)
	}

	// Set PID limit if specified
	if agentConfig.Resources.PidsLimit > 0 {
		args = append(args, "--pids-limit", fmt.Sprintf("%d", agentConfig.Resources.PidsLimit))
	}

	// Set network mode if specified
	if agentConfig.Runtime.NetworkMode != "" {
		args = append(args, "--network", agentConfig.Runtime.NetworkMode)
	}

	// Set privileged mode if specified
	if agentConfig.Runtime.Privileged {
		args = append(args, "--privileged")
	}

	// Add capabilities if specified
	for _, cap := range agentConfig.Runtime.Capabilities {
		args = append(args, "--cap-add", cap)
	}

	// Add devices if specified
	for _, device := range agentConfig.Runtime.Devices {
		args = append(args, "--device", device)
	}

	// Auto-remove if specified
	if agentConfig.Runtime.AutoRemove {
		args = append(args, "--rm")
	}

	// Add image and command
	args = append(args, container.Config.Image)
	if len(agentConfig.Command) > 0 {
		args = append(args, agentConfig.Command...)
	} else if len(container.Config.Cmd) > 0 {
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

// CleanupLaForgeContainers removes all LaForge containers

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
