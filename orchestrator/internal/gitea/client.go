package gitea

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"code.gitea.io/sdk/gitea"
	"github.com/tom/laforge/orchestrator/internal/types"
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

// GetSDKClient returns the underlying Gitea SDK client
func (c *Client) GetSDKClient() *gitea.Client {
	return c.client
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
	Branch   string
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

	// Extract branch name
	branch := ""
	if pr.Head != nil {
		branch = pr.Head.Ref
	}

	return &PRDetails{
		HeadSHA:  pr.Head.Sha,
		HeadRepo: headRepo,
		Branch:   branch,
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

// PostComment posts a comment to a PR
func (c *Client) PostComment(ctx context.Context, repository string, prNumber int, body string) error {
	// Parse repository into owner and repo
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository format: %s (expected owner/repo)", repository)
	}
	owner, repo := parts[0], parts[1]

	slog.Debug("posting comment to PR",
		"owner", owner,
		"repo", repo,
		"pr_number", prNumber,
	)

	// Create comment using Gitea SDK
	_, resp, err := c.client.CreateIssueComment(owner, repo, int64(prNumber), gitea.CreateIssueCommentOption{
		Body: body,
	})
	if err != nil {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		return fmt.Errorf("failed to post comment (HTTP %d) to %s/%s#%d: %w",
			statusCode, owner, repo, prNumber, err)
	}

	slog.Info("posted comment to PR", "repository", repository, "pr_number", prNumber)
	return nil
}

// PostReviewComments posts file-level comments as a PR review
func (c *Client) PostReviewComments(ctx context.Context, repository string, prNumber int, commitSHA string, comments []*types.FileComment) error {
	// Parse repository into owner and repo
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository format: %s (expected owner/repo)", repository)
	}
	owner, repo := parts[0], parts[1]

	fileComments := comments

	slog.Debug("posting review comments to PR",
		"owner", owner,
		"repo", repo,
		"pr_number", prNumber,
		"comment_count", len(fileComments),
	)

	// Post each comment as a separate review
	// Note: Gitea API doesn't support batch review comments the same way GitHub does
	for _, fc := range fileComments {
		if fc.File == "" || fc.Line == 0 || fc.Comment == "" {
			slog.Warn("skipping invalid file comment", "file", fc.File, "line", fc.Line)
			continue
		}

		// Create review with comment
		_, resp, err := c.client.CreatePullReview(owner, repo, int64(prNumber), gitea.CreatePullReviewOptions{
			State: gitea.ReviewStateComment,
			Body:  fc.Comment,
			Comments: []gitea.CreatePullReviewComment{
				{
					Path:       fc.File,
					Body:       fc.Comment,
					NewLineNum: int64(fc.Line),
				},
			},
		})
		if err != nil {
			statusCode := 0
			if resp != nil {
				statusCode = resp.StatusCode
			}
			slog.Error("failed to post review comment",
				"file", fc.File,
				"line", fc.Line,
				"status", statusCode,
				"error", err,
			)
			// Continue with other comments even if one fails
			continue
		}

		slog.Debug("posted review comment", "file", fc.File, "line", fc.Line)
	}

	slog.Info("posted review comments to PR", "repository", repository, "pr_number", prNumber)
	return nil
}

// UpdatePRAssignees updates the assignees for a PR
func (c *Client) UpdatePRAssignees(ctx context.Context, repository string, prNumber int, assignees []string) error {
	// Parse repository into owner and repo
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository format: %s (expected owner/repo)", repository)
	}
	owner, repo := parts[0], parts[1]

	slog.Debug("updating PR assignees",
		"owner", owner,
		"repo", repo,
		"pr_number", prNumber,
		"assignees", assignees,
	)

	// Update assignees using EditIssue (PRs are issues in Gitea)
	_, resp, err := c.client.EditIssue(owner, repo, int64(prNumber), gitea.EditIssueOption{
		Assignees: assignees,
	})
	if err != nil {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		return fmt.Errorf("failed to update assignees (HTTP %d) for %s/%s#%d: %w",
			statusCode, owner, repo, prNumber, err)
	}

	slog.Info("updated PR assignees", "repository", repository, "pr_number", prNumber, "assignees", assignees)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
