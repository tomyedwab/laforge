package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tomyedwab/laforge/lib/errors"
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
	if !IsGitRepository(projectDir) {
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

	// Note: Branch cleanup is now handled by the automerge functionality in runStep
	// This function only removes the worktree, leaving branch management to the merge process

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

// CreateTempWorktreeWithStep creates a temporary worktree with a step-based branch name
func CreateTempWorktreeWithStep(projectDir string, stepNumber int) (*Worktree, error) {
	// Generate a branch name based on step number
	branchName := fmt.Sprintf("step-S%d", stepNumber)

	// Create a temporary directory for the worktree
	tempDir, err := os.MkdirTemp("", fmt.Sprintf("laforge-worktree-step-S%d-", stepNumber))
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

// IsGitRepository checks if the given directory is a git repository
func IsGitRepository(dir string) bool {
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

// GetCurrentCommitSHA returns the current commit SHA for the given repository
func GetCurrentCommitSHA(repoDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current commit SHA: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
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

// ResetToCommit resets the repository to the specified commit SHA
func ResetToCommit(repoDir string, commitSHA string) error {
	// Check if git is available
	if err := exec.Command("git", "--version").Run(); err != nil {
		return fmt.Errorf("git is not available: %w", err)
	}

	// Verify the repository directory is a git repository
	if !IsGitRepository(repoDir) {
		return fmt.Errorf("directory is not a git repository: %s", repoDir)
	}

	// Verify the commit SHA exists
	cmd := exec.Command("git", "rev-parse", "--verify", commitSHA+"^{commit}")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("commit SHA %s does not exist: %w", commitSHA, err)
	}

	// Reset to the specified commit
	cmd = exec.Command("git", "reset", "--hard", commitSHA)
	cmd.Dir = repoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reset to commit %s: %w\nOutput: %s", commitSHA, err, string(output))
	}

	return nil
}

// MergeBranch merges source branch into target branch
func MergeBranch(repoDir string, sourceBranch string, targetBranch string, message string) error {
	// Check if git is available
	if err := exec.Command("git", "--version").Run(); err != nil {
		return fmt.Errorf("git is not available: %w", err)
	}

	// Verify the repository directory is a git repository
	if !IsGitRepository(repoDir) {
		return fmt.Errorf("directory is not a git repository: %s", repoDir)
	}

	// Get current branch to restore later
	originalBranch, err := GetCurrentBranch(repoDir)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Switch to target branch if not already on it
	if originalBranch != targetBranch {
		if err := SwitchBranch(repoDir, targetBranch); err != nil {
			return fmt.Errorf("failed to switch to target branch %s: %w", targetBranch, err)
		}
		defer func() {
			// Restore original branch
			_ = SwitchBranch(repoDir, originalBranch)
		}()
	}

	// Perform the merge
	cmd := exec.Command("git", "merge", sourceBranch, "-m", message)
	cmd.Dir = repoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		// Check if it's a merge conflict
		if strings.Contains(string(output), "CONFLICT") {
			return errors.NewGitMergeConflictError(
				fmt.Errorf("merge conflict detected: %w\nOutput: %s", err, string(output)),
				sourceBranch, targetBranch,
			)
		}
		return fmt.Errorf("failed to merge %s into %s: %w\nOutput: %s", sourceBranch, targetBranch, err, string(output))
	}

	return nil
}

// DeleteBranch deletes a branch if it exists and is not current
func DeleteBranch(repoDir string, branchName string) error {
	// Check if git is available
	if err := exec.Command("git", "--version").Run(); err != nil {
		return fmt.Errorf("git is not available: %w", err)
	}

	// Verify the repository directory is a git repository
	if !IsGitRepository(repoDir) {
		return fmt.Errorf("directory is not a git repository: %s", repoDir)
	}

	// Check if branch exists
	exists, err := BranchExists(repoDir, branchName)
	if err != nil {
		return fmt.Errorf("failed to check if branch exists: %w", err)
	}
	if !exists {
		return nil // Branch doesn't exist, nothing to do
	}

	// Check if it's the current branch
	currentBranch, err := GetCurrentBranch(repoDir)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	if currentBranch == branchName {
		return fmt.Errorf("cannot delete current branch %s", branchName)
	}

	// Delete the branch
	cmd := exec.Command("git", "branch", "-D", branchName)
	cmd.Dir = repoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete branch %s: %w\nOutput: %s", branchName, err, string(output))
	}

	return nil
}

// GetCurrentBranch returns the currently checked out branch
func GetCurrentBranch(repoDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// SwitchBranch switches to the specified branch
func SwitchBranch(repoDir string, branchName string) error {
	cmd := exec.Command("git", "checkout", branchName)
	cmd.Dir = repoDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to switch to branch %s: %w\nOutput: %s", branchName, err, string(output))
	}

	return nil
}

// BranchExists checks if a branch exists
func BranchExists(repoDir string, branchName string) (bool, error) {
	cmd := exec.Command("git", "branch", "--list", branchName)
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to list branches: %w", err)
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}
