package gitea

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"code.gitea.io/sdk/gitea"
)

// Status represents the status of a commit
type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusSuccess
	StatusFailure
)

const (
	// Context for commit status
	statusContext = "laforge/agent"
)

// Client wraps the Gitea SDK client
type Client struct {
	client *gitea.Client
}

// NewClient creates a new Gitea client
func NewClient(url, token string) (*Client, error) {
	client, err := gitea.NewClient(url, gitea.SetToken(token))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gitea client: %w", err)
	}

	return &Client{client: client}, nil
}

// UpdateStatus updates the commit status for a PR
func (c *Client) UpdateStatus(ctx context.Context, repository, sha string, status Status) error {
	// Parse repository into owner and repo
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository format: %s (expected owner/repo)", repository)
	}
	owner, repo := parts[0], parts[1]

	// Convert our status to Gitea status
	var giteaStatus gitea.StatusState
	var description string

	switch status {
	case StatusPending:
		giteaStatus = gitea.StatusPending
		description = "Job is queued and waiting to be processed"
	case StatusRunning:
		giteaStatus = gitea.StatusPending
		description = "Agent is running"
	case StatusSuccess:
		giteaStatus = gitea.StatusSuccess
		description = "Job completed successfully"
	case StatusFailure:
		giteaStatus = gitea.StatusFailure
		description = "Job failed"
	default:
		return fmt.Errorf("unknown status: %d", status)
	}

	// Debug logging before API call
	slog.Debug("creating commit status",
		"owner", owner,
		"repo", repo,
		"sha", sha,
		"status", giteaStatus,
		"context", statusContext,
	)

	// Create the status
	_, resp, err := c.client.CreateStatus(owner, repo, sha, gitea.CreateStatusOption{
		State:       giteaStatus,
		Context:     statusContext,
		Description: description,
	})

	if err != nil {
		// Add more context to the error
		statusCode := 0
		body := ""
		if resp != nil {
			statusCode = resp.StatusCode
		}
		return fmt.Errorf("failed to create status (HTTP %d) for %s/%s@%s: %w (response: %s)",
			statusCode, owner, repo, sha[:min(7, len(sha))], err, body)
	}

	slog.Info("updated commit status",
		"repository", repository,
		"sha", sha,
		"status", giteaStatus,
		"description", description,
	)

	return nil
}

// PRDetails contains basic PR information needed for job processing
type PRDetails struct {
	HeadSHA  string
	HeadRepo string
}

// GetPullRequest retrieves basic PR information from Gitea
func (c *Client) GetPullRequest(ctx context.Context, repository string, prNumber int) (*PRDetails, error) {
	// Parse repository into owner and repo
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository format: %s (expected owner/repo)", repository)
	}
	owner, repo := parts[0], parts[1]

	// Debug logging before API call
	slog.Debug("fetching pull request",
		"owner", owner,
		"repo", repo,
		"pr_number", prNumber,
	)

	// Fetch PR from Gitea
	pr, resp, err := c.client.GetPullRequest(owner, repo, int64(prNumber))
	if err != nil {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		return nil, fmt.Errorf("failed to get PR (HTTP %d) for %s/%s#%d: %w",
			statusCode, owner, repo, prNumber, err)
	}

	// Extract head repository
	headRepo := repository // default to base repo
	if pr.Head != nil && pr.Head.Repository != nil {
		headRepo = pr.Head.Repository.FullName
	}

	return &PRDetails{
		HeadSHA:  pr.Head.Sha,
		HeadRepo: headRepo,
	}, nil
}

// GetPullRequestAssignees retrieves the list of assignees for a PR
func (c *Client) GetPullRequestAssignees(ctx context.Context, repository string, prNumber int) ([]string, error) {
	// Parse repository into owner and repo
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository format: %s (expected owner/repo)", repository)
	}
	owner, repo := parts[0], parts[1]

	// Fetch PR from Gitea
	pr, resp, err := c.client.GetPullRequest(owner, repo, int64(prNumber))
	if err != nil {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		return nil, fmt.Errorf("failed to get PR (HTTP %d) for %s/%s#%d: %w",
			statusCode, owner, repo, prNumber, err)
	}

	// Extract assignee usernames
	assignees := make([]string, 0, len(pr.Assignees))
	for _, assignee := range pr.Assignees {
		if assignee != nil {
			assignees = append(assignees, assignee.UserName)
		}
	}

	return assignees, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
