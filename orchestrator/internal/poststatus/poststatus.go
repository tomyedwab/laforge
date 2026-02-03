package poststatus

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/tom/laforge/orchestrator/internal/gitea"
	"github.com/tom/laforge/orchestrator/internal/types"
	"gopkg.in/yaml.v3"
)

// StatusData represents the structure of .pr/status.yaml
type StatusData struct {
	Status       string                `yaml:"status"`
	FileComments []*types.FileComment  `yaml:"file_comments"`
	Unassign     bool                  `yaml:"unassign"`
}

// PostStatus parses and posts status updates from .pr/status.yaml
// commitSHA should be the HEAD commit SHA after the agent's changes were committed
func PostStatus(ctx context.Context, statusYAML []byte, commitSHA, repository string, prNumber int, giteaClient *gitea.Client) error {
	if len(statusYAML) == 0 {
		slog.Info("no status file found, skipping status post")
		return nil
	}

	// Parse YAML
	var statusData StatusData
	if err := yaml.Unmarshal(statusYAML, &statusData); err != nil {
		// If parsing fails, treat entire content as status message
		slog.Warn("failed to parse status.yaml as YAML, using as plain text", "error", err)
		statusData = StatusData{
			Status:       string(statusYAML),
			FileComments: []*types.FileComment{},
			Unassign:     false,
		}
	}

	// Replace {{COMMIT_SHA}} placeholder with actual SHA
	if commitSHA != "" {
		statusData.Status = strings.ReplaceAll(statusData.Status, "{{COMMIT_SHA}}", commitSHA)
		for i := range statusData.FileComments {
			statusData.FileComments[i].Comment = strings.ReplaceAll(
				statusData.FileComments[i].Comment,
				"{{COMMIT_SHA}}",
				commitSHA,
			)
		}
	}

	// Post status comment if present
	if statusData.Status != "" {
		if err := giteaClient.PostComment(ctx, repository, prNumber, statusData.Status); err != nil {
			return fmt.Errorf("failed to post status comment: %w", err)
		}
		slog.Info("posted status comment to PR")
	}

	// Post file comments if present
	if len(statusData.FileComments) > 0 {
		if err := giteaClient.PostReviewComments(ctx, repository, prNumber, commitSHA, statusData.FileComments); err != nil {
			return fmt.Errorf("failed to post review comments: %w", err)
		}
		slog.Info("posted file comments to PR", "count", len(statusData.FileComments))
	}

	// Handle unassign if requested
	if statusData.Unassign {
		if err := giteaClient.UpdatePRAssignees(ctx, repository, prNumber, []string{}); err != nil {
			return fmt.Errorf("failed to unassign from PR: %w", err)
		}
		slog.Info("unassigned from PR")
	}

	slog.Info("status processing completed successfully")
	return nil
}
