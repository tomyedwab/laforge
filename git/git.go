package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree represents a git worktree
type Worktree struct {
	Path        string
	Branch      string
	OriginalDir string
}

// CreateWorktree creates a temporary git worktree for the given project
func CreateWorktree(projectDir string, worktreeDir string, branchName string) (*Worktree, error) {
	// Check if git is available
	if err := exec.Command("git", "--version").Run(); err != nil {
		return nil, fmt.Errorf("git is not available: %w", err)
	}

	// Verify the project directory is a git repository
	if !isGitRepository(projectDir) {
		return nil, fmt.Errorf("project directory is not a git repository: %s", projectDir)
	}

	// Create the worktree directory if it doesn't exist
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create worktree directory: %w", err)
	}

	// Create the worktree
	cmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreeDir)
	cmd.Dir = projectDir
	if output, err := cmd.CombinedOutput(); err != nil {
		// Clean up on error
		os.RemoveAll(worktreeDir)
		return nil, fmt.Errorf("failed to create worktree: %w\nOutput: %s", err, string(output))
	}

	return &Worktree{
		Path:        worktreeDir,
		Branch:      branchName,
		OriginalDir: projectDir,
	}, nil
}

// RemoveWorktree removes a git worktree and cleans up the associated branch
func RemoveWorktree(worktree *Worktree) error {
	// Remove the worktree
	cmd := exec.Command("git", "worktree", "remove", worktree.Path)
	cmd.Dir = worktree.OriginalDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove worktree: %w\nOutput: %s", err, string(output))
	}

	// Remove the worktree directory if it still exists
	if _, err := os.Stat(worktree.Path); err == nil {
		if err := os.RemoveAll(worktree.Path); err != nil {
			return fmt.Errorf("failed to remove worktree directory: %w", err)
		}
	}

	/* TODO this needs to be done _after_ the changes are merged to main
	// Prune the branch (if it exists and is not the current branch)
	if !isCurrentBranch(worktree.OriginalDir, worktree.Branch) {
		cmd = exec.Command("git", "branch", "-D", worktree.Branch)
		cmd.Dir = worktree.OriginalDir
		if output, err := cmd.CombinedOutput(); err != nil {
			// It's okay if the branch doesn't exist
			if !strings.Contains(string(output), "not found") {
				return fmt.Errorf("failed to delete branch: %w\nOutput: %s", err, string(output))
			}
		}
	}
	*/

	return nil
}

// GetWorktrees returns all active worktrees for a repository
func GetWorktrees(repoDir string) ([]*Worktree, error) {
	// Check if git is available
	if err := exec.Command("git", "--version").Run(); err != nil {
		return nil, fmt.Errorf("git is not available: %w", err)
	}

	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	return parseWorktreeList(string(output), repoDir), nil
}

// CreateTempWorktree creates a temporary worktree in the system's temp directory
func CreateTempWorktree(projectDir string, branchPrefix string) (*Worktree, error) {
	// Generate a unique branch name
	branchName := fmt.Sprintf("%s-%d", branchPrefix, os.Getpid())

	// Create a temporary directory for the worktree
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("laforge-worktree-%s-", branchPrefix))
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	worktree, err := CreateWorktree(projectDir, tempDir, branchName)
	if err != nil {
		// Clean up temp directory on error
		os.RemoveAll(tempDir)
		return nil, err
	}

	return worktree, nil
}

// isGitRepository checks if the given directory is a git repository
func isGitRepository(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return true
	}

	// Also check if it's a git worktree (has .git file pointing to actual repo)
	gitFile := filepath.Join(dir, ".git")
	if info, err := os.Stat(gitFile); err == nil && !info.IsDir() {
		return true
	}

	return false
}

// isCurrentBranch checks if the given branch is the currently checked out branch
func isCurrentBranch(repoDir string, branchName string) bool {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	currentBranch := strings.TrimSpace(string(output))
	return currentBranch == branchName
}

// parseWorktreeList parses the output of 'git worktree list --porcelain'
func parseWorktreeList(output string, repoDir string) []*Worktree {
	var worktrees []*Worktree
	var currentWorktree *Worktree

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			if currentWorktree != nil {
				worktrees = append(worktrees, currentWorktree)
			}
			path := strings.TrimPrefix(line, "worktree ")
			currentWorktree = &Worktree{
				Path:        path,
				OriginalDir: repoDir,
			}
		} else if strings.HasPrefix(line, "branch ") && currentWorktree != nil {
			branchRef := strings.TrimPrefix(line, "branch ")
			// Extract branch name from ref (refs/heads/branch-name)
			if strings.HasPrefix(branchRef, "refs/heads/") {
				currentWorktree.Branch = strings.TrimPrefix(branchRef, "refs/heads/")
			} else {
				currentWorktree.Branch = branchRef
			}
		}
	}

	if currentWorktree != nil {
		worktrees = append(worktrees, currentWorktree)
	}

	return worktrees
}

// CleanupWorktrees removes all worktrees that match the given prefix
func CleanupWorktrees(repoDir string, prefix string) error {
	worktrees, err := GetWorktrees(repoDir)
	if err != nil {
		return err
	}

	var lastErr error
	for _, worktree := range worktrees {
		if strings.HasPrefix(worktree.Branch, prefix) {
			if err := RemoveWorktree(worktree); err != nil {
				lastErr = err
				// Continue with other worktrees even if one fails
			}
		}
	}

	return lastErr
}
