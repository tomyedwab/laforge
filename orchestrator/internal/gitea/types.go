package gitea

import (
	giteasdk "code.gitea.io/sdk/gitea"
)

// NOTE: These webhook payload types are copied from the Gitea source code (tag 1.25.4)
// because the Gitea SDK does not export PullRequestPayload or IssueCommentPayload types.
// They embed SDK types for nested structures (Issue, PullRequest, Repository, User, Comment)
// to ensure type safety and consistency with Gitea's webhook wire format.
// See: https://github.com/go-gitea/gitea/blob/v1.25.4/modules/structs/hook.go

type IssueCommentPayload struct {
	// The action performed on the comment (created, edited, deleted)
	Action string `json:"action"`
	// The issue that the comment belongs to
	Issue *giteasdk.Issue `json:"issue"`
	// The pull request if the comment is on a pull request
	PullRequest *giteasdk.PullRequest `json:"pull_request,omitempty"`
	// The comment that was acted upon
	Comment *giteasdk.Comment `json:"comment"`
	// Changes made to the comment (for edit actions)
	//Changes *giteasdk.ChangesPayload `json:"changes,omitempty"`
	// The repository containing the issue/pull request
	Repository *giteasdk.Repository `json:"repository"`
	// The user who performed the action
	Sender *giteasdk.User `json:"sender"`
	// Whether this comment is on a pull request
	IsPull bool `json:"is_pull"`
}

type ReviewPayload struct {
	// The type of review (approved, rejected, comment)
	Type string `json:"type"`
	// The content/body of the review
	Content string `json:"content"`
}

type PullRequestPayload struct {
	// The action performed on the pull request
	Action string `json:"action"`
	// The index number of the pull request
	Index int64 `json:"number"`
	// Changes made to the pull request (for edit actions)
	//Changes *giteasdk.ChangesPayload `json:"changes,omitempty"`
	// The pull request that was acted upon
	PullRequest *giteasdk.PullRequest `json:"pull_request"`
	// The reviewer that was requested (for review request actions)
	RequestedReviewer *giteasdk.User `json:"requested_reviewer"`
	// The repository containing the pull request
	Repository *giteasdk.Repository `json:"repository"`
	// The user who performed the action
	Sender *giteasdk.User `json:"sender"`
	// The commit ID related to the pull request action
	CommitID string `json:"commit_id"`
	// The review information (for review actions)
	Review *ReviewPayload `json:"review"`
}
