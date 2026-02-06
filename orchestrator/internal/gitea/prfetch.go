package gitea

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"code.gitea.io/sdk/gitea"
)

// Attachment represents a downloaded attachment from PR comments
type Attachment struct {
	URL          string // Original URL
	LocalPath    string // Relative path in .pr/attachments/
	Filename     string // Just the filename
	Content      []byte // File content
	OriginalLink string // Original markdown link
	NewLink      string // New markdown link pointing to local file
}

// PRHistory contains the formatted PR history and attachments
type PRHistory struct {
	Markdown    string       // Formatted markdown content
	Attachments []Attachment // Downloaded attachments
}

// Fetcher fetches PR history and attachments from Gitea
type Fetcher struct {
	client  *gitea.Client
	baseURL string // Base URL for Gitea (e.g., "http://gitea:3000")
	token   string // Authentication token
}

// NewFetcher creates a new PR history fetcher
func NewFetcher(client *gitea.Client, baseURL, token string) *Fetcher {
	return &Fetcher{
		client:  client,
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
	}
}

// FetchPRHistory fetches and formats the complete PR history
func (f *Fetcher) FetchPRHistory(ctx context.Context, repository string, prNumber int) (*PRHistory, error) {
	// Parse repository into owner and repo
	owner, repo, err := parseRepository(repository)
	if err != nil {
		return nil, err
	}

	slog.Info("fetching PR history",
		"repository", repository,
		"pr_number", prNumber,
	)

	// Fetch PR data
	pr, _, err := f.client.GetPullRequest(owner, repo, int64(prNumber))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch PR: %w", err)
	}

	// Fetch comments
	comments, _, err := f.client.ListIssueComments(owner, repo, int64(prNumber), gitea.ListIssueCommentOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch comments: %w", err)
	}

	// Fetch reviews
	reviews, _, err := f.client.ListPullReviews(owner, repo, int64(prNumber), gitea.ListPullReviewsOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch reviews: %w", err)
	}

	// Fetch review comments for each review
	reviewsWithComments := make([]reviewWithComments, 0, len(reviews))
	for _, review := range reviews {
		reviewComments, _, err := f.client.ListPullReviewComments(owner, repo, int64(prNumber), review.ID)
		if err != nil {
			slog.Warn("failed to fetch review comments",
				"review_id", review.ID,
				"error", err,
			)
			reviewComments = []*gitea.PullReviewComment{}
		}
		reviewsWithComments = append(reviewsWithComments, reviewWithComments{
			Review:   review,
			Comments: reviewComments,
		})
	}

	// Extract all attachments from all content
	allAttachments := make([]Attachment, 0)

	// Process PR description
	if pr.Body != "" {
		attachments := f.extractAttachments(pr.Body)
		allAttachments = append(allAttachments, attachments...)
	}

	// Process comments
	for _, comment := range comments {
		if comment.Body != "" {
			attachments := f.extractAttachments(comment.Body)
			allAttachments = append(allAttachments, attachments...)
		}
	}

	// Process reviews
	for _, review := range reviewsWithComments {
		if review.Review.Body != "" {
			attachments := f.extractAttachments(review.Review.Body)
			allAttachments = append(allAttachments, attachments...)
		}
		// Process review comments
		for _, comment := range review.Comments {
			if comment.Body != "" {
				attachments := f.extractAttachments(comment.Body)
				allAttachments = append(allAttachments, attachments...)
			}
		}
	}

	// Download all attachments
	slog.Info("downloading attachments", "count", len(allAttachments))
	for i := range allAttachments {
		if err := f.downloadAttachment(ctx, &allAttachments[i]); err != nil {
			slog.Error("failed to download attachment",
				"url", allAttachments[i].URL,
				"error", err,
			)
			// Continue with other attachments
		}
	}

	// Build markdown with rewritten attachment links
	markdown := f.formatMarkdown(pr, comments, reviewsWithComments, allAttachments)

	return &PRHistory{
		Markdown:    markdown,
		Attachments: allAttachments,
	}, nil
}

type reviewWithComments struct {
	Review   *gitea.PullReview
	Comments []*gitea.PullReviewComment
}

// extractAttachments extracts attachment URLs from markdown text
func (f *Fetcher) extractAttachments(text string) []Attachment {
	// Match markdown image syntax: ![alt](url)
	attachmentRegex := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	matches := attachmentRegex.FindAllStringSubmatch(text, -1)

	attachments := make([]Attachment, 0)
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}

		altText := match[1]
		attachmentURL := match[2]

		// Check if this is an attachment URL (starts with /attachments/)
		if !strings.HasPrefix(attachmentURL, "/attachments/") {
			continue
		}

		// Generate filename from alt text or URL
		filename := altText
		if filename == "" {
			filename = path.Base(attachmentURL)
		}

		attachments = append(attachments, Attachment{
			URL:          attachmentURL,
			Filename:     filename,
			LocalPath:    ".pr/attachments/" + filename,
			OriginalLink: fmt.Sprintf("![%s](%s)", altText, attachmentURL),
			NewLink:      fmt.Sprintf("![%s](%s)", altText, ".pr/attachments/"+filename),
		})
	}

	return attachments
}

// downloadAttachment downloads an attachment from Gitea
func (f *Fetcher) downloadAttachment(ctx context.Context, attachment *Attachment) error {
	// Construct full URL
	fullURL := f.baseURL + attachment.URL

	slog.Debug("downloading attachment",
		"url", fullURL,
		"filename", attachment.Filename,
	)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication
	req.Header.Set("Authorization", "token "+f.token)

	// Execute request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read content with size limit to prevent DoS
	content, err := io.ReadAll(io.LimitReader(resp.Body, 100<<20)) // 100MB limit
	if err != nil {
		return fmt.Errorf("failed to read content: %w", err)
	}

	attachment.Content = content
	return nil
}

// formatMarkdown formats the PR data into markdown
func (f *Fetcher) formatMarkdown(
	pr *gitea.PullRequest,
	comments []*gitea.Comment,
	reviews []reviewWithComments,
	attachments []Attachment,
) string {
	var sb strings.Builder

	// PR header
	sb.WriteString(fmt.Sprintf("# PR #%d: %s\n\n", pr.Index, pr.Title))
	sb.WriteString(fmt.Sprintf("**Author:** %s\n", pr.Poster.UserName))
	sb.WriteString(fmt.Sprintf("**Branch:** %s\n", pr.Head.Ref))
	sb.WriteString(fmt.Sprintf("**Created:** %s\n\n", pr.Created.Format(time.RFC3339)))

	// PR description
	sb.WriteString("## PR Description\n")
	prBody := f.rewriteAttachmentLinks(pr.Body, attachments)
	sb.WriteString(prBody + "\n\n")

	// Conversation comments
	sb.WriteString("## Conversation Comments\n")
	for _, comment := range comments {
		sb.WriteString(fmt.Sprintf("\n**%s** (%s):\n",
			comment.Poster.UserName,
			comment.Created.Format(time.RFC3339),
		))
		commentBody := f.rewriteAttachmentLinks(comment.Body, attachments)
		sb.WriteString(commentBody + "\n")
	}

	// Reviews
	sb.WriteString("\n## Reviews\n")
	for _, review := range reviews {
		sb.WriteString(fmt.Sprintf("\n### %s - %s (%s)\n",
			review.Review.Reviewer.UserName,
			review.Review.State,
			review.Review.Submitted.Format(time.RFC3339),
		))

		if review.Review.Body != "" {
			reviewBody := f.rewriteAttachmentLinks(review.Review.Body, attachments)
			sb.WriteString(reviewBody + "\n")
		}

		// Review comments
		if len(review.Comments) > 0 {
			sb.WriteString("\n#### Review Comments:\n")
			for _, comment := range review.Comments {
				sb.WriteString(fmt.Sprintf("\n**%s** on `%s`",
					comment.Reviewer.UserName,
					comment.Path,
				))
				if comment.LineNum > 0 {
					sb.WriteString(fmt.Sprintf(" (line %d)", comment.LineNum))
				}
				commentBody := f.rewriteAttachmentLinks(comment.Body, attachments)
				sb.WriteString(":\n" + commentBody + "\n")
			}
		}
	}

	return sb.String()
}

// rewriteAttachmentLinks rewrites attachment links in text to point to local files
func (f *Fetcher) rewriteAttachmentLinks(text string, attachments []Attachment) string {
	result := text
	for _, att := range attachments {
		result = strings.ReplaceAll(result, att.OriginalLink, att.NewLink)
	}
	return result
}

// GetCloneURL constructs the git clone URL for a repository
func GetCloneURL(baseURL, token, repository, sha string) (string, error) {
	// Parse base URL
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	// Inject token into URL for authentication
	// Format: https://token@host/repo.git
	u.User = url.User(token)
	u.Path = path.Join(u.Path, repository+".git")

	return u.String(), nil
}
