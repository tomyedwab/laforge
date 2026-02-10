package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// Client wraps the Docker client for workspace operations
type Client struct {
	docker            *client.Client
	jobName           string
	networkName       string
	anthropicProxyURL string
	hasAnthropicProxy bool
	logsVolumeName    string
}

// NewClient creates a new Docker client
func NewClient(jobName string) (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Client{
		docker:  cli,
		jobName: jobName,
	}, nil
}

// SetNetworkConfig sets the network configuration for agent containers
func (c *Client) SetNetworkConfig(networkName string) {
	c.networkName = networkName
}

// SetAnthropicProxy sets the Anthropic proxy configuration for agent containers
func (c *Client) SetAnthropicProxy(proxyURL string) {
	c.anthropicProxyURL = proxyURL
	c.hasAnthropicProxy = true
}

// SetLogsVolume sets the logs volume name for agent containers
func (c *Client) SetLogsVolume(volumeName string) {
	c.logsVolumeName = volumeName
}

// Close closes the Docker client connection
func (c *Client) Close() error {
	return c.docker.Close()
}

// PullImage pulls a Docker image from a registry
func (c *Client) PullImage(ctx context.Context, imageName string) error {
	slog.Info("pulling Docker image", "image", imageName)

	// Pull the image
	reader, err := c.docker.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer reader.Close()

	// Consume the output to ensure the pull completes
	// The ImagePull API streams the pull progress, so we need to read it
	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return fmt.Errorf("failed to read image pull output for %s: %w", imageName, err)
	}

	slog.Info("image pulled successfully", "image", imageName)
	return nil
}

// CreateVolume creates a new Docker volume
func (c *Client) CreateVolume(ctx context.Context, name string) error {
	slog.Info("creating Docker volume", "name", name)

	_, err := c.docker.VolumeCreate(ctx, volume.CreateOptions{
		Name: name,
		Labels: map[string]string{
			"laforge.managed": "true",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create volume: %w", err)
	}

	return nil
}

// DeleteVolume deletes a Docker volume
func (c *Client) DeleteVolume(ctx context.Context, name string) error {
	slog.Info("deleting Docker volume", "name", name)

	if err := c.docker.VolumeRemove(ctx, name, true); err != nil {
		return fmt.Errorf("failed to delete volume: %w", err)
	}

	return nil
}

// RunCleanupContainer runs a container to remove the .pr directory
func (c *Client) RunCleanupContainer(ctx context.Context, volumeName, imageName string) error {
	slog.Info("running cleanup container", "volume", volumeName)

	// Create container config to remove .pr directory
	containerConfig := &container.Config{
		Image:      imageName,
		Entrypoint: []string{"sh", "-c"},
		Cmd: []string{`
			cd /workspace/repo
			if [ -d ".pr" ]; then
				rm -rf .pr
				echo "Removed .pr directory"
			else
				echo ".pr directory does not exist"
			fi
		`},
		WorkingDir: "/workspace",
	}

	// Mount the volume
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: volumeName,
				Target: "/workspace",
			},
		},
		AutoRemove: true,
	}

	// Create the container
	resp, err := c.docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, c.jobName+"-cleanup")
	if err != nil {
		return fmt.Errorf("failed to create cleanup container: %w", err)
	}

	containerID := resp.ID
	slog.Debug("created cleanup container", "id", containerID)

	// Start the container
	if err := c.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start cleanup container: %w", err)
	}

	// Wait for container to finish
	if err := c.waitForContainer(ctx, containerID, "cleanup"); err != nil {
		return err
	}

	logs, _ := c.getContainerLogs(ctx, containerID)
	slog.Info("cleanup container completed successfully", "logs", logs)
	return nil
}

// RunInitContainer runs a container to initialize the workspace with git clone
func (c *Client) RunInitContainer(ctx context.Context, volumeName, cloneURL, sha, imageName string) error {
	slog.Info("running init container",
		"volume", volumeName,
		"sha", sha,
		"image", imageName,
	)

	// Create container config
	// The init container will shallow clone the repository at the specified
	// commit SHA
	containerConfig := &container.Config{
		Image: imageName,
		Cmd: []string{
			"clone", "--depth", "1", "--revision", sha, cloneURL, "repo",
		},
		WorkingDir: "/workspace",
	}

	// Mount the volume
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: volumeName,
				Target: "/workspace",
			},
		},
		AutoRemove: true, // Automatically remove container after it exits
	}

	// Join the Docker network if configured
	if c.networkName != "" {
		hostConfig.NetworkMode = container.NetworkMode(c.networkName)
		slog.Debug("agent container joining network", "network", c.networkName)
	}

	// Create the container
	resp, err := c.docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, c.jobName+"-init")
	if err != nil {
		return fmt.Errorf("failed to create init container: %w", err)
	}

	containerID := resp.ID
	slog.Debug("created init container", "id", containerID)

	// Start the container
	if err := c.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start init container: %w", err)
	}

	// Wait for container to finish
	if err := c.waitForContainer(ctx, containerID, "init"); err != nil {
		return err
	}

	logs, _ := c.getContainerLogs(ctx, containerID)
	slog.Info("init container completed successfully", "logs", logs)
	return nil
}

// CopyFilesToVolume copies files into a volume using Docker's native tar-based API
// This is secure and handles binary files correctly, unlike shell-based approaches
func (c *Client) CopyFilesToVolume(ctx context.Context, volumeName string, files map[string][]byte, imageName string) error {
	slog.Info("copying files to volume",
		"volume", volumeName,
		"file_count", len(files),
	)

	if len(files) == 0 {
		return nil
	}

	// Create a temporary container with the volume mounted
	// We'll use this container as a target for CopyToContainer
	containerConfig := &container.Config{
		Image:      imageName,
		Cmd:        []string{"sleep", "3600"}, // Sleep long enough for us to copy files
		WorkingDir: "/workspace",
	}

	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: volumeName,
				Target: "/workspace",
			},
		},
	}

	// Create the temporary container
	resp, err := c.docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, c.jobName+"-up")
	if err != nil {
		return fmt.Errorf("failed to create temporary container for file copy: %w", err)
	}

	containerID := resp.ID
	slog.Debug("created temporary file copy container", "id", containerID)

	// Ensure we clean up the container when done
	defer func() {
		removeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := c.docker.ContainerRemove(removeCtx, containerID, container.RemoveOptions{Force: true}); err != nil {
			slog.Warn("failed to remove temporary file copy container", "id", containerID, "error", err)
		}
	}()

	// Start the container so we can copy files into it
	if err := c.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start temporary container: %w", err)
	}

	// Create a tar archive containing all the files
	tarBuffer, err := createTarArchive(files)
	if err != nil {
		return fmt.Errorf("failed to create tar archive: %w", err)
	}

	// Copy the tar archive to the container at /workspace/repo
	// The Docker API will automatically extract the tar
	if err := c.docker.CopyToContainer(ctx, containerID, "/workspace/repo", tarBuffer, container.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("failed to copy files to container: %w", err)
	}

	slog.Info("files copied successfully")
	return nil
}

// createTarArchive creates a tar archive containing the given files
func createTarArchive(files map[string][]byte) (*bytes.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for path, content := range files {
		// Clean the path and ensure it's relative
		cleanPath := filepath.Clean(path)
		if filepath.IsAbs(cleanPath) {
			cleanPath = strings.TrimPrefix(cleanPath, "/")
		}

		// Write tar header
		header := &tar.Header{
			Name: cleanPath,
			Mode: 0644,
			Size: int64(len(content)),
		}

		if err := tw.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("failed to write tar header for %s: %w", path, err)
		}

		// Write file content
		if _, err := tw.Write(content); err != nil {
			return nil, fmt.Errorf("failed to write tar content for %s: %w", path, err)
		}
	}

	// Flush the tar writer
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close tar writer: %w", err)
	}

	return bytes.NewReader(buf.Bytes()), nil
}

// RunAgentContainer runs the agent container with the workspace volume mounted
func (c *Client) RunAgentContainer(ctx context.Context, volumeName, imageName, promptType, model, bashProxyToken string) error {
	slog.Info("running agent container",
		"volume", volumeName,
		"image", imageName,
		"prompt", promptType,
		"model", model,
		"has_bash_proxy", bashProxyToken != "",
	)

	// Build environment variables
	env := []string{
		"PROMPTNAME=" + promptType,
		"MODELNAME=" + model,
	}

	if c.logsVolumeName != "" {
		env = append(env, "LAFORGE_LOG_FILE=/logs/"+c.jobName+".jsonl")
	}

	// Add Anthropic proxy configuration if enabled
	if c.hasAnthropicProxy {
		env = append(env, "ANTHROPIC_BASE_URL="+c.anthropicProxyURL)
		env = append(env, "CLAUDE_CODE_OAUTH_TOKEN=dummy")
		slog.Info("agent container configured with Anthropic proxy", "proxy_url", c.anthropicProxyURL)
	}

	// Add bash proxy configuration if token is provided
	if bashProxyToken != "" {
		env = append(env, "LAFORGE_BASH_PROXY_TOKEN="+bashProxyToken)
		env = append(env, "LAFORGE_BASH_PROXY_URL=http://orchestrator:8080")
		slog.Info("agent container configured with bash proxy")
	}

	containerConfig := &container.Config{
		Image:      imageName,
		Cmd:        []string{"/bin/run.sh"},
		Env:        env,
		WorkingDir: "/workspace/repo",
	}

	// Mount the volume
	mounts := []mount.Mount{
		{
			Type:   mount.TypeVolume,
			Source: volumeName,
			Target: "/workspace",
		},
	}

	// Add logs volume if configured
	if c.logsVolumeName != "" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: c.logsVolumeName,
			Target: "/logs",
		})
	}

	hostConfig := &container.HostConfig{
		Mounts:     mounts,
		AutoRemove: true,
		// Grant NET_ADMIN capability to allow iptables firewall initialization
		CapAdd: []string{"NET_ADMIN"},
	}

	// Join the Docker network if configured
	if c.networkName != "" {
		hostConfig.NetworkMode = container.NetworkMode(c.networkName)
		slog.Debug("agent container joining network", "network", c.networkName)
	}

	// Create the container
	resp, err := c.docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, c.jobName+"-agent")
	if err != nil {
		return fmt.Errorf("failed to create agent container: %w", err)
	}

	containerID := resp.ID
	slog.Info("created agent container", "id", containerID)

	// Start the container
	if err := c.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start agent container: %w", err)
	}

	// Wait for container to finish
	if err := c.waitForContainer(ctx, containerID, "agent"); err != nil {
		return err
	}

	slog.Info("agent container completed successfully")
	return nil
}

// CopyFilesFromVolume extracts files from a volume back to the host
// Returns a map of file paths to file contents, or an error
func (c *Client) CopyFilesFromVolume(ctx context.Context, volumeName string, filePaths []string, imageName string) (map[string][]byte, error) {
	slog.Info("copying files from volume",
		"volume", volumeName,
		"file_count", len(filePaths),
	)

	if len(filePaths) == 0 {
		return make(map[string][]byte), nil
	}

	// Create a temporary container with the volume mounted
	containerConfig := &container.Config{
		Image:      imageName,
		Cmd:        []string{"sleep", "3600"},
		WorkingDir: "/workspace",
	}

	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: volumeName,
				Target: "/workspace",
			},
		},
	}

	// Create the temporary container
	resp, err := c.docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, c.jobName+"-dn")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary container for file extraction: %w", err)
	}

	containerID := resp.ID
	slog.Debug("created temporary file extraction container", "id", containerID)

	// Ensure we clean up the container when done
	defer func() {
		removeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := c.docker.ContainerRemove(removeCtx, containerID, container.RemoveOptions{Force: true}); err != nil {
			slog.Warn("failed to remove temporary file extraction container", "id", containerID, "error", err)
		}
	}()

	// Start the container so we can copy files from it
	if err := c.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start temporary container: %w", err)
	}

	// Extract each file
	result := make(map[string][]byte)
	for _, filePath := range filePaths {
		// Construct full path in container
		fullPath := "/workspace/repo/" + filePath

		// Copy file from container
		reader, _, err := c.docker.CopyFromContainer(ctx, containerID, fullPath)
		if err != nil {
			// File might not exist, which is OK for optional files like status.yaml
			slog.Debug("failed to copy file from container (file may not exist)", "file", filePath, "error", err)
			continue
		}

		// Extract content from tar archive
		content, err := extractTarFile(reader, filePath)
		reader.Close()
		if err != nil {
			slog.Warn("failed to extract file from tar", "file", filePath, "error", err)
			continue
		}

		result[filePath] = content
		slog.Debug("extracted file from volume", "file", filePath, "size", len(content))
	}

	slog.Info("files extracted successfully", "count", len(result))
	return result, nil
}

// extractTarFile extracts a single file from a tar archive reader
func extractTarFile(reader io.Reader, fileName string) ([]byte, error) {
	tr := tar.NewReader(reader)

	// Find the file in the tar archive
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("file %s not found in tar archive", fileName)
		}
		if err != nil {
			return nil, fmt.Errorf("error reading tar archive: %w", err)
		}

		// Check if this is the file we want (match basename)
		if filepath.Base(header.Name) == filepath.Base(fileName) || header.Name == fileName {
			// Read the file content
			content, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("error reading file content: %w", err)
			}
			return content, nil
		}
	}
}

// CommitAndPushChanges commits and pushes changes from the volume to the PR branch
// Returns the HEAD commit SHA after committing, or empty string if no changes
func (c *Client) CommitAndPushChanges(ctx context.Context, volumeName, imageName, giteaURL, giteaToken, repository, branch, botUsername, botEmail string, commitMessage []byte) (string, error) {
	slog.Info("committing and pushing changes",
		"volume", volumeName,
		"repository", repository,
		"branch", branch,
	)

	// Determine commit message
	message := "Agent: updating PR status"
	if len(commitMessage) > 0 {
		message = string(commitMessage)
	}

	// Derive base git URL from API URL (remove /api/v1 suffix)
	gitBaseURL := strings.TrimSuffix(giteaURL, "/api/v1")
	gitBaseURL = strings.TrimSuffix(gitBaseURL, "/api")

	// Construct authenticated push URL
	// Format: http://username:token@host/repo.git
	pushURL := gitBaseURL
	if strings.HasPrefix(pushURL, "http://") {
		pushURL = "http://" + botUsername + ":" + giteaToken + "@" + strings.TrimPrefix(gitBaseURL, "http://")
	} else if strings.HasPrefix(pushURL, "https://") {
		pushURL = "https://" + botUsername + ":" + giteaToken + "@" + strings.TrimPrefix(gitBaseURL, "https://")
	}
	pushURL = pushURL + "/" + repository + ".git"

	// Build git command script
	script := fmt.Sprintf(`#!/bin/sh
set -e
cd /workspace/repo

# Configure git
git config user.name %s
git config user.email %s

# Stage all changes except special files
git add .
git reset HEAD .pr/history.md .pr/status.yaml .pr/status.md .pr/commit.md 2>/dev/null || true

# Check if there are changes to commit
if git diff --cached --quiet; then
    echo "NO_CHANGES"
    exit 0
fi

# Commit changes
git commit -m %s

# Push to remote
git push %s HEAD:%s

# Output the HEAD SHA with a clear marker
echo "NEW_HEAD_SHA: $(git rev-parse HEAD)"
`, shellQuote(botUsername), shellQuote(botEmail), shellQuote(message), shellQuote(pushURL), shellQuote(branch))

	// Create container config
	// Note: Override the entrypoint to use sh instead of git, since the git image
	// has git as its default entrypoint which would prepend "git" to our commands
	containerConfig := &container.Config{
		Image:      imageName,
		Entrypoint: []string{"sh", "-c"},
		Cmd:        []string{script},
		WorkingDir: "/workspace/repo",
	}

	// Mount the volume
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: volumeName,
				Target: "/workspace",
			},
		},
		// Note: Don't use AutoRemove here because we need to get logs after the container exits
	}

	// Join the Docker network if configured
	if c.networkName != "" {
		hostConfig.NetworkMode = container.NetworkMode(c.networkName)
		slog.Debug("agent container joining network", "network", c.networkName)
	}

	// Create the container
	resp, err := c.docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, c.jobName+"-commit")
	if err != nil {
		return "", fmt.Errorf("failed to create git commit container: %w", err)
	}

	containerID := resp.ID
	slog.Debug("created git commit container", "id", containerID)

	// Ensure container is removed after we're done
	defer func() {
		if err := c.docker.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
			slog.Warn("failed to remove git commit container", "id", containerID, "error", err)
		}
	}()

	// Start the container
	if err := c.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start git commit container: %w", err)
	}

	// Wait for container to finish
	if err := c.waitForContainer(ctx, containerID, "git commit"); err != nil {
		return "", err
	}

	// Get output to check if there were changes and get the SHA
	logs, err := c.getContainerLogs(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("failed to get git commit logs: %w", err)
	}

	// Check if there were no changes
	if strings.Contains(logs, "NO_CHANGES") {
		slog.Info("no changes to commit")
		return "", nil
	}

	// Extract HEAD SHA from logs (look for "NEW_HEAD_SHA: <sha>" marker)
	headSHA := ""
	for _, line := range strings.Split(logs, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "NEW_HEAD_SHA: ") {
			headSHA = strings.TrimPrefix(line, "NEW_HEAD_SHA: ")
			break
		}
	}

	// Validate we found the SHA
	if headSHA == "" {
		slog.Warn("could not find NEW_HEAD_SHA marker in logs", "logs", logs)
		return "", fmt.Errorf("could not find NEW_HEAD_SHA marker in output")
	}

	// Validate SHA format (should be 40 hex characters)
	if len(headSHA) != 40 {
		slog.Warn("unexpected HEAD SHA format", "sha", headSHA, "logs", logs)
		return "", fmt.Errorf("unexpected HEAD SHA format: %s", headSHA)
	}

	slog.Info("changes committed and pushed successfully", "head_sha", headSHA)
	return headSHA, nil
}

// shellQuote properly quotes a string for use in a shell script
func shellQuote(s string) string {
	// Use single quotes and escape any single quotes in the string
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// waitForContainer waits for a container to finish and returns an error if the
// container exits with a non-zero status code. This correctly handles the Docker
// SDK's ContainerWait semantics where errCh may fire with nil before statusCh.
func (c *Client) waitForContainer(ctx context.Context, containerID, name string) error {
	statusCh, errCh := c.docker.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for %s container: %w", name, err)
		}
		// errCh returned nil — still need to check exit status
		status := <-statusCh
		if status.StatusCode != 0 {
			logs, _ := c.getContainerLogs(ctx, containerID)
			return fmt.Errorf("%s container exited with code %d: %s", name, status.StatusCode, logs)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			logs, _ := c.getContainerLogs(ctx, containerID)
			return fmt.Errorf("%s container exited with code %d: %s", name, status.StatusCode, logs)
		}
	}
	return nil
}

// getContainerLogs retrieves logs from a container
func (c *Client) getContainerLogs(ctx context.Context, containerID string) (string, error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	}

	logs, err := c.docker.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", err
	}
	defer logs.Close()

	// Use stdcopy to demultiplex the logs
	var stdout, stderr strings.Builder
	_, err = stdcopy.StdCopy(&stdout, &stderr, logs)
	if err != nil && err != io.EOF {
		return "", err
	}

	return stdout.String() + stderr.String(), nil
}

// StartDevcontainer starts a persistent devcontainer with the workspace volume mounted
// Returns the container ID
func (c *Client) StartDevcontainer(ctx context.Context, volumeName, devcontainerImage string) (string, error) {
	slog.Info("starting persistent devcontainer",
		"volume", volumeName,
		"image", devcontainerImage,
	)

	// Create container config with a long-running process
	// We use 'sleep infinity' to keep the container running
	containerConfig := &container.Config{
		Image:      devcontainerImage,
		Entrypoint: []string{"sleep"},
		Cmd:        []string{"infinity"},
		WorkingDir: "/workspace/repo",
	}

	// Mount the volume
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: volumeName,
				Target: "/workspace",
			},
		},
		// Don't use AutoRemove - we'll manually remove it after stopping
	}

	// Join the Docker network if configured
	if c.networkName != "" {
		hostConfig.NetworkMode = container.NetworkMode(c.networkName)
		slog.Debug("devcontainer joining network", "network", c.networkName)
	}

	// Create the container
	resp, err := c.docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, c.jobName+"-devcontainer")
	if err != nil {
		return "", fmt.Errorf("failed to create devcontainer: %w", err)
	}

	containerID := resp.ID
	slog.Debug("created devcontainer", "id", containerID)

	// Start the container
	if err := c.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start devcontainer: %w", err)
	}

	slog.Info("persistent devcontainer started successfully", "id", containerID)
	return containerID, nil
}

// ExecDevcontainerCommand executes a bash command in a running devcontainer using docker exec
// Returns stdout, stderr, and exit code
func (c *Client) ExecDevcontainerCommand(ctx context.Context, containerID, command string) (string, string, int, error) {
	slog.Info("executing command in devcontainer",
		"container_id", containerID,
		"command", command,
	)

	// Create exec configuration
	execConfig := container.ExecOptions{
		Cmd:          []string{"bash", "-c", command},
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   "/workspace/repo",
	}

	// Create the exec instance
	execResp, err := c.docker.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", "", 1, fmt.Errorf("failed to create exec: %w", err)
	}

	execID := execResp.ID
	slog.Debug("created exec instance", "exec_id", execID)

	// Attach to the exec instance to capture output
	attachResp, err := c.docker.ContainerExecAttach(ctx, execID, container.ExecStartOptions{})
	if err != nil {
		return "", "", 1, fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer attachResp.Close()

	// Read stdout and stderr from the exec
	var stdout, stderr strings.Builder
	_, err = stdcopy.StdCopy(&stdout, &stderr, attachResp.Reader)
	if err != nil && err != io.EOF {
		return "", "", 1, fmt.Errorf("failed to read exec output: %w", err)
	}

	// Wait for the exec to complete and get exit code
	// We need to inspect the exec to get the exit code
	var exitCode int
	for {
		inspectResp, err := c.docker.ContainerExecInspect(ctx, execID)
		if err != nil {
			return "", "", 1, fmt.Errorf("failed to inspect exec: %w", err)
		}

		if !inspectResp.Running {
			exitCode = inspectResp.ExitCode
			break
		}

		// Small sleep to avoid busy-waiting
		time.Sleep(100 * time.Millisecond)
	}

	slog.Info("devcontainer command completed", "exit_code", exitCode)
	return stdout.String(), stderr.String(), exitCode, nil
}

// StopDevcontainer stops and removes a running devcontainer
func (c *Client) StopDevcontainer(ctx context.Context, containerID string) error {
	slog.Info("stopping devcontainer", "container_id", containerID)

	// Stop the container with a timeout
	timeout := 10 // 10 seconds
	stopOptions := container.StopOptions{
		Timeout: &timeout,
	}

	if err := c.docker.ContainerStop(ctx, containerID, stopOptions); err != nil {
		// Container might already be stopped, log but continue to remove
		slog.Warn("failed to stop devcontainer (may already be stopped)", "error", err)
	}

	// Remove the container
	if err := c.docker.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove devcontainer: %w", err)
	}

	slog.Info("devcontainer stopped and removed successfully")
	return nil
}

// RunDevcontainerCommand executes a bash command in a devcontainer with the workspace volume mounted
// DEPRECATED: Use StartDevcontainer + ExecDevcontainerCommand + StopDevcontainer instead
// Returns stdout, stderr, and exit code
func (c *Client) RunDevcontainerCommand(ctx context.Context, volumeName, devcontainerImage, command string) (string, string, int, error) {
	slog.Info("running command in devcontainer",
		"volume", volumeName,
		"image", devcontainerImage,
		"command", command,
	)

	// Create container config to run the command
	containerConfig := &container.Config{
		Image:      devcontainerImage,
		Entrypoint: []string{"bash", "-c"},
		Cmd:        []string{command},
		WorkingDir: "/workspace/repo",
	}

	// Mount the volume
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeVolume,
				Source: volumeName,
				Target: "/workspace",
			},
		},
		AutoRemove: true,
	}

	// Join the Docker network if configured
	if c.networkName != "" {
		hostConfig.NetworkMode = container.NetworkMode(c.networkName)
		slog.Debug("devcontainer joining network", "network", c.networkName)
	}

	// Create the container
	resp, err := c.docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, c.jobName+"-devcontainer")
	if err != nil {
		return "", "", 1, fmt.Errorf("failed to create devcontainer: %w", err)
	}

	containerID := resp.ID
	slog.Debug("created devcontainer", "id", containerID)

	// Start the container
	if err := c.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return "", "", 1, fmt.Errorf("failed to start devcontainer: %w", err)
	}

	// Wait for container to finish
	statusCh, errCh := c.docker.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)

	var exitCode int64
	select {
	case err := <-errCh:
		if err != nil {
			return "", "", 1, fmt.Errorf("error waiting for devcontainer: %w", err)
		}
		// errCh returned nil — still need to check exit status
		status := <-statusCh
		exitCode = status.StatusCode
	case status := <-statusCh:
		exitCode = status.StatusCode
	}

	// Get stdout and stderr separately
	stdout, stderr, err := c.getContainerLogsWithSeparation(ctx, containerID)
	if err != nil {
		return "", "", int(exitCode), fmt.Errorf("failed to get devcontainer logs: %w", err)
	}

	slog.Info("devcontainer command completed", "exit_code", exitCode)
	return stdout, stderr, int(exitCode), nil
}

// getContainerLogsWithSeparation retrieves logs from a container with stdout and stderr separated
func (c *Client) getContainerLogsWithSeparation(ctx context.Context, containerID string) (string, string, error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	}

	logs, err := c.docker.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", "", err
	}
	defer logs.Close()

	// Use stdcopy to demultiplex the logs
	var stdout, stderr strings.Builder
	_, err = stdcopy.StdCopy(&stdout, &stderr, logs)
	if err != nil && err != io.EOF {
		return "", "", err
	}

	return stdout.String(), stderr.String(), nil
}
