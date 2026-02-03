package notify

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
)

// Client handles sending notifications to ntfy
type Client struct {
	endpoint        string
	giteaExternalURL string
	httpClient      *http.Client
}

// Config holds configuration for the notify client
type Config struct {
	Endpoint string // e.g., "http://ntfy:80"
	GiteaURL string // e.g., "https://gitea.example.com" (externally-accessible URL for links)
}

// NewClient creates a new notification client
func NewClient(cfg Config) *Client {
	return &Client{
		endpoint:        cfg.Endpoint,
		giteaExternalURL: cfg.GiteaURL,
		httpClient:      &http.Client{},
	}
}

// NotifySuccess sends a success notification for a completed PR job
func (c *Client) NotifySuccess(ctx context.Context, repository string, prNumber int) error {
	title := fmt.Sprintf("Laforge run complete: PR %d", prNumber)
	body := fmt.Sprintf("Click to view PR %d", prNumber)
	clickURL := fmt.Sprintf("%s/%s/pulls/%d", c.giteaExternalURL, repository, prNumber)

	return c.sendNotification(ctx, title, body, clickURL, "white_check_mark", "")
}

// NotifyFailure sends a failure notification for a failed PR job (after retries exhausted)
func (c *Client) NotifyFailure(ctx context.Context, repository string, prNumber int) error {
	title := fmt.Sprintf("ERROR - Laforge Agent Failed: PR %d", prNumber)
	body := "Job failed with no retries remaining."
	clickURL := fmt.Sprintf("%s/%s/pulls/%d", c.giteaExternalURL, repository, prNumber)

	return c.sendNotification(ctx, title, body, clickURL, "", "high")
}

// sendNotification sends a notification to ntfy
func (c *Client) sendNotification(ctx context.Context, title, body, clickURL, tags, priority string) error {
	url := fmt.Sprintf("%s/laforge", c.endpoint)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBufferString(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Title", title)
	req.Header.Set("Click", clickURL)
	if tags != "" {
		req.Header.Set("Tags", tags)
	}
	if priority != "" {
		req.Header.Set("Priority", priority)
	}

	// Send the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ntfy returned error status: %d", resp.StatusCode)
	}

	slog.Debug("notification sent successfully",
		"title", title,
		"url", clickURL,
		"status_code", resp.StatusCode,
	)

	return nil
}
