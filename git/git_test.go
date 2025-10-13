package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCreateWorktree(t *testing.T) {
	// Skip if git is not available
	if err := exec.Command("git", "--version").Run(); err != nil {
		t.Skip("git is not available")
	}

	// Create a temporary directory for the test repository
	tempDir, err := os.MkdirTemp("", "laforge-git-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repoDir := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo directory: %v", err)
	}

	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize git repository: %v", err)
	}

	// Configure git user
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git user.name: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git user.email: %v", err)
	}

	// Create a test file and commit it
	testFile := filepath.Join(repoDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add files to git: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create initial commit: %v", err)
	}

	// Create worktree
	worktreeDir := filepath.Join(tempDir, "worktree")
	worktree, err := CreateWorktree(repoDir, worktreeDir, "test-branch")
	if err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}

	// Verify worktree was created
	if worktree.Path != worktreeDir {
		t.Errorf("Expected worktree path %s, got %s", worktreeDir, worktree.Path)
	}

	if worktree.Branch != "test-branch" {
		t.Errorf("Expected branch name test-branch, got %s", worktree.Branch)
	}

	if worktree.OriginalDir != repoDir {
		t.Errorf("Expected original dir %s, got %s", repoDir, worktree.OriginalDir)
	}

	// Verify worktree directory exists and contains the test file
	if _, err := os.Stat(worktreeDir); err != nil {
		t.Errorf("Worktree directory does not exist: %v", err)
	}

	worktreeTestFile := filepath.Join(worktreeDir, "test.txt")
	if _, err := os.Stat(worktreeTestFile); err != nil {
		t.Errorf("Test file not found in worktree: %v", err)
	}

	// Clean up
	if err := RemoveWorktree(worktree); err != nil {
		t.Errorf("Failed to remove worktree: %v", err)
	}

	// Verify worktree was removed
	if _, err := os.Stat(worktreeDir); !os.IsNotExist(err) {
		t.Errorf("Worktree directory still exists after removal")
	}
}

func TestCreateTempWorktree(t *testing.T) {
	// Skip if git is not available
	if err := exec.Command("git", "--version").Run(); err != nil {
		t.Skip("git is not available")
	}

	// Create a temporary directory for the test repository
	tempDir, err := os.MkdirTemp("", "laforge-git-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repoDir := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo directory: %v", err)
	}

	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize git repository: %v", err)
	}

	// Configure git user
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git user.name: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git user.email: %v", err)
	}

	// Create a test file and commit it
	testFile := filepath.Join(repoDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add files to git: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create initial commit: %v", err)
	}

	// Create temporary worktree
	worktree, err := CreateTempWorktree(repoDir, "test")
	if err != nil {
		t.Fatalf("Failed to create temporary worktree: %v", err)
	}

	// Verify worktree was created with proper prefix
	if !filepath.IsAbs(worktree.Path) {
		t.Errorf("Expected absolute path for temporary worktree, got %s", worktree.Path)
	}

	if !filepath.HasPrefix(filepath.Base(worktree.Path), "laforge-worktree-test-") {
		t.Errorf("Expected worktree path to have prefix laforge-worktree-test-, got %s", worktree.Path)
	}

	if !filepath.HasPrefix(worktree.Branch, "test-") {
		t.Errorf("Expected branch name to have prefix test-, got %s", worktree.Branch)
	}

	// Clean up
	if err := RemoveWorktree(worktree); err != nil {
		t.Errorf("Failed to remove temporary worktree: %v", err)
	}
}

func TestIsGitRepository(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "laforge-git-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test non-git directory
	if isGitRepository(tempDir) {
		t.Errorf("Expected non-git directory to return false")
	}

	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Skip("git is not available")
	}

	// Test git directory
	if !isGitRepository(tempDir) {
		t.Errorf("Expected git directory to return true")
	}
}

func TestGetWorktrees(t *testing.T) {
	// Skip if git is not available
	if err := exec.Command("git", "--version").Run(); err != nil {
		t.Skip("git is not available")
	}

	// Create a temporary directory for the test repository
	tempDir, err := os.MkdirTemp("", "laforge-git-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	repoDir := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("Failed to create repo directory: %v", err)
	}

	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize git repository: %v", err)
	}

	// Configure git user
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git user.name: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git user.email: %v", err)
	}

	// Create a test file and commit it
	testFile := filepath.Join(repoDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add files to git: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create initial commit: %v", err)
	}

	// Get initial worktrees (should be empty except for main worktree)
	worktrees, err := GetWorktrees(repoDir)
	if err != nil {
		t.Fatalf("Failed to get worktrees: %v", err)
	}

	// Should have at least the main worktree
	if len(worktrees) == 0 {
		t.Errorf("Expected at least one worktree (main), got %d", len(worktrees))
	}

	// Create a new worktree
	worktreeDir := filepath.Join(tempDir, "worktree")
	worktree, err := CreateWorktree(repoDir, worktreeDir, "test-branch")
	if err != nil {
		t.Fatalf("Failed to create worktree: %v", err)
	}

	// Get worktrees again
	worktrees, err = GetWorktrees(repoDir)
	if err != nil {
		t.Fatalf("Failed to get worktrees after creation: %v", err)
	}

	// Should now have the main worktree plus our new one
	found := false
	for _, wt := range worktrees {
		if wt.Path == worktreeDir {
			found = true
			if wt.Branch != "test-branch" {
				t.Errorf("Expected branch name test-branch, got %s", wt.Branch)
			}
		}
	}

	if !found {
		t.Errorf("Created worktree not found in worktree list")
	}

	// Clean up
	if err := RemoveWorktree(worktree); err != nil {
		t.Errorf("Failed to remove worktree: %v", err)
	}
}
