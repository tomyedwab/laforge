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
	docker *client.Client
}

// NewClient creates a new Docker client
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Client{docker: cli}, nil
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

	// Create the container
	resp, err := c.docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
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
	statusCh, errCh := c.docker.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for init container: %w", err)
		}
	case status := <-statusCh:
		// Get logs to help debug
		if status.StatusCode != 0 {
			logs, _ := c.getContainerLogs(ctx, containerID)
			return fmt.Errorf("init container exited with code %d: %s", status.StatusCode, logs)
		}
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
	resp, err := c.docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
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
	defer tw.Close()

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
func (c *Client) RunAgentContainer(ctx context.Context, volumeName, imageName string, sleepDuration time.Duration) error {
	slog.Info("running agent container",
		"volume", volumeName,
		"image", imageName,
		"sleep_duration", sleepDuration,
	)

	// For now, just run a sleep command as a placeholder
	containerConfig := &container.Config{
		Image:      imageName,
		Cmd:        []string{"sh", "-c", fmt.Sprintf("sleep %d", int(sleepDuration.Seconds()))},
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

	// Create the container
	resp, err := c.docker.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
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
	statusCh, errCh := c.docker.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for agent container: %w", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			logs, _ := c.getContainerLogs(ctx, containerID)
			return fmt.Errorf("agent container exited with code %d: %s", status.StatusCode, logs)
		}
	}

	slog.Info("agent container completed successfully")
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
