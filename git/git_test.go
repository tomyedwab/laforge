package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tomyedwab/laforge/errors"
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

func TestCreateTempWorktreeWithStep(t *testing.T) {
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

	// Configure git user (required for commits)
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

	// Test with different step numbers
	testCases := []int{1, 5, 10, 999}

	for _, stepNum := range testCases {
		t.Run(fmt.Sprintf("step_S%d", stepNum), func(t *testing.T) {
			// Create temporary worktree with step number
			worktree, err := CreateTempWorktreeWithStep(repoDir, stepNum)
			if err != nil {
				t.Fatalf("Failed to create temporary worktree with step S%d: %v", stepNum, err)
			}
			defer func() {
				// Clean up worktree
				if err := RemoveWorktree(worktree); err != nil {
					t.Errorf("Failed to remove temporary worktree: %v", err)
				}
			}()

			// Verify worktree was created with proper step-based branch name
			expectedBranch := fmt.Sprintf("step-S%d", stepNum)
			if worktree.Branch != expectedBranch {
				t.Errorf("Expected branch name %s, got %s", expectedBranch, worktree.Branch)
			}

			// Verify worktree directory contains expected pattern
			expectedDirPattern := fmt.Sprintf("laforge-worktree-step-S%d-", stepNum)
			if !strings.Contains(worktree.Path, expectedDirPattern) {
				t.Errorf("Worktree path %s does not contain expected pattern %s", worktree.Path, expectedDirPattern)
			}

			// Verify worktree was actually created by checking if directory exists
			if _, err := os.Stat(worktree.Path); os.IsNotExist(err) {
				t.Errorf("Worktree directory does not exist: %s", worktree.Path)
			}

			// Verify it's a valid git repository
			if !IsGitRepository(worktree.Path) {
				t.Errorf("Worktree path is not a git repository: %s", worktree.Path)
			}
		})
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
	if IsGitRepository(tempDir) {
		t.Errorf("Expected non-git directory to return false")
	}

	// Initialize git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Skip("git is not available")
	}

	// Test git directory
	if !IsGitRepository(tempDir) {
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

func TestBranchOperations(t *testing.T) {
	// Skip if git is not available
	if err := exec.Command("git", "--version").Run(); err != nil {
		t.Skip("git is not available")
	}

	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "laforge-git-branch-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Initialize a git repository
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to initialize git repository: %v", err)
	}

	// Configure git user for testing
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git user.name: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to configure git user.email: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add test file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create initial commit: %v", err)
	}

	t.Run("BranchExists", func(t *testing.T) {
		// Test main branch exists
		exists, err := BranchExists(tempDir, "main")
		if err != nil {
			t.Errorf("BranchExists failed: %v", err)
		}
		if !exists {
			t.Error("Expected main branch to exist")
		}

		// Test non-existent branch
		exists, err = BranchExists(tempDir, "nonexistent")
		if err != nil {
			t.Errorf("BranchExists failed: %v", err)
		}
		if exists {
			t.Error("Expected nonexistent branch to not exist")
		}
	})

	t.Run("GetCurrentBranch", func(t *testing.T) {
		branch, err := GetCurrentBranch(tempDir)
		if err != nil {
			t.Errorf("GetCurrentBranch failed: %v", err)
		}
		if branch != "main" {
			t.Errorf("Expected current branch to be 'main', got '%s'", branch)
		}
	})

	t.Run("SwitchBranch and CreateBranch", func(t *testing.T) {
		// Create a new branch by switching to it
		cmd = exec.Command("git", "checkout", "-b", "test-branch")
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to create test branch: %v", err)
		}

		// Verify we're on the new branch
		branch, err := GetCurrentBranch(tempDir)
		if err != nil {
			t.Errorf("GetCurrentBranch failed: %v", err)
		}
		if branch != "test-branch" {
			t.Errorf("Expected current branch to be 'test-branch', got '%s'", branch)
		}

		// Switch back to main
		if err := SwitchBranch(tempDir, "main"); err != nil {
			t.Errorf("SwitchBranch failed: %v", err)
		}

		// Verify we're back on main
		branch, err = GetCurrentBranch(tempDir)
		if err != nil {
			t.Errorf("GetCurrentBranch failed: %v", err)
		}
		if branch != "main" {
			t.Errorf("Expected current branch to be 'main' after switch, got '%s'", branch)
		}
	})

	t.Run("DeleteBranch", func(t *testing.T) {
		// Create a test branch
		cmd = exec.Command("git", "checkout", "-b", "delete-me")
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to create branch to delete: %v", err)
		}

		// Switch back to main
		if err := SwitchBranch(tempDir, "main"); err != nil {
			t.Fatalf("Failed to switch to main: %v", err)
		}

		// Delete the branch
		if err := DeleteBranch(tempDir, "delete-me"); err != nil {
			t.Errorf("DeleteBranch failed: %v", err)
		}

		// Verify it's gone
		exists, err := BranchExists(tempDir, "delete-me")
		if err != nil {
			t.Errorf("BranchExists failed: %v", err)
		}
		if exists {
			t.Error("Expected deleted branch to not exist")
		}
	})

	t.Run("DeleteBranch_CurrentBranch", func(t *testing.T) {
		// Create a test branch and switch to it
		cmd = exec.Command("git", "checkout", "-b", "current-branch")
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to create current branch: %v", err)
		}

		// Try to delete the current branch (should fail)
		err := DeleteBranch(tempDir, "current-branch")
		if err == nil {
			t.Error("Expected DeleteBranch to fail when trying to delete current branch")
		}
	})

	t.Run("MergeBranch_Success", func(t *testing.T) {
		// Create a feature branch
		cmd = exec.Command("git", "checkout", "-b", "feature")
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to create feature branch: %v", err)
		}

		// Make a change on the feature branch
		if err := os.WriteFile(testFile, []byte("feature content"), 0644); err != nil {
			t.Fatalf("Failed to update test file: %v", err)
		}

		cmd = exec.Command("git", "add", "test.txt")
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to add changes: %v", err)
		}

		cmd = exec.Command("git", "commit", "-m", "Feature commit")
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to commit feature changes: %v", err)
		}

		// Switch back to main
		if err := SwitchBranch(tempDir, "main"); err != nil {
			t.Fatalf("Failed to switch to main: %v", err)
		}

		// Merge feature into main
		if err := MergeBranch(tempDir, "feature", "main", "Merge feature into main"); err != nil {
			t.Errorf("MergeBranch failed: %v", err)
		}

		// Verify the merge by checking file content
		content, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read test file: %v", err)
		}
		if string(content) != "feature content" {
			t.Errorf("Expected file content to be 'feature content', got '%s'", string(content))
		}
	})

	t.Run("MergeBranch_Conflict", func(t *testing.T) {
		// Create a conflict branch
		cmd = exec.Command("git", "checkout", "-b", "conflict")
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to create conflict branch: %v", err)
		}

		// Make a conflicting change
		if err := os.WriteFile(testFile, []byte("conflict content"), 0644); err != nil {
			t.Fatalf("Failed to update test file: %v", err)
		}

		cmd = exec.Command("git", "add", "test.txt")
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to add changes: %v", err)
		}

		cmd = exec.Command("git", "commit", "-m", "Conflict commit")
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to commit conflict changes: %v", err)
		}

		// Switch back to main
		if err := SwitchBranch(tempDir, "main"); err != nil {
			t.Fatalf("Failed to switch to main: %v", err)
		}

		// Make a change on main that conflicts
		if err := os.WriteFile(testFile, []byte("main content"), 0644); err != nil {
			t.Fatalf("Failed to update test file on main: %v", err)
		}

		cmd = exec.Command("git", "add", "test.txt")
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to add changes on main: %v", err)
		}

		cmd = exec.Command("git", "commit", "-m", "Main commit")
		cmd.Dir = tempDir
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to commit main changes: %v", err)
		}

		// Try to merge conflict branch (should fail with merge conflict)
		err := MergeBranch(tempDir, "conflict", "main", "Merge conflict into main")
		if err == nil {
			t.Error("Expected MergeBranch to fail with merge conflict")
		}

		// Check if it's a merge conflict error
		if !errors.IsErrorType(err, errors.ErrGitMergeConflict) {
			t.Errorf("Expected merge conflict error type, got: %v", err)
		}
	})
}

func TestResetToCommit(t *testing.T) {
	// Skip if git is not available
	if err := exec.Command("git", "--version").Run(); err != nil {
		t.Skip("git is not available")
	}

	// Create a temporary directory for the test repository
	tempDir, err := os.MkdirTemp("", "laforge-git-reset-test-")
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

	// Create initial commit
	testFile1 := filepath.Join(repoDir, "test1.txt")
	if err := os.WriteFile(testFile1, []byte("initial content"), 0644); err != nil {
		t.Fatalf("Failed to create test file 1: %v", err)
	}

	cmd = exec.Command("git", "add", "test1.txt")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add test file 1: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit test file 1: %v", err)
	}

	// Get the initial commit SHA
	initialSHA, err := GetCurrentCommitSHA(repoDir)
	if err != nil {
		t.Fatalf("Failed to get initial commit SHA: %v", err)
	}

	// Create second commit
	testFile2 := filepath.Join(repoDir, "test2.txt")
	if err := os.WriteFile(testFile2, []byte("second content"), 0644); err != nil {
		t.Fatalf("Failed to create test file 2: %v", err)
	}

	cmd = exec.Command("git", "add", "test2.txt")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add test file 2: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Second commit")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit test file 2: %v", err)
	}

	// Verify both files exist
	if _, err := os.Stat(testFile1); os.IsNotExist(err) {
		t.Fatal("test1.txt should exist before reset")
	}
	if _, err := os.Stat(testFile2); os.IsNotExist(err) {
		t.Fatal("test2.txt should exist before reset")
	}

	// Reset to initial commit
	if err := ResetToCommit(repoDir, initialSHA); err != nil {
		t.Fatalf("Failed to reset to initial commit: %v", err)
	}

	// Verify we're back to initial state
	currentSHA, err := GetCurrentCommitSHA(repoDir)
	if err != nil {
		t.Fatalf("Failed to get current commit SHA after reset: %v", err)
	}

	if currentSHA != initialSHA {
		t.Errorf("Current SHA %s does not match initial SHA %s after reset", currentSHA, initialSHA)
	}

	// Verify test2.txt no longer exists (reset should have removed it)
	if _, err := os.Stat(testFile2); !os.IsNotExist(err) {
		t.Error("test2.txt should not exist after reset to initial commit")
	}

	// Verify test1.txt still exists
	if _, err := os.Stat(testFile1); os.IsNotExist(err) {
		t.Error("test1.txt should still exist after reset")
	}
}

func TestResetToCommitInvalidSHA(t *testing.T) {
	// Skip if git is not available
	if err := exec.Command("git", "--version").Run(); err != nil {
		t.Skip("git is not available")
	}

	// Create a temporary directory for the test repository
	tempDir, err := os.MkdirTemp("", "laforge-git-reset-invalid-test-")
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

	// Try to reset to non-existent commit
	err = ResetToCommit(repoDir, "nonexistentsha123456789")
	if err == nil {
		t.Error("Expected error when resetting to non-existent commit SHA")
	}
}

func TestResetToCommitNonRepo(t *testing.T) {
	// Create a temporary directory that's not a git repository
	tempDir, err := os.MkdirTemp("", "laforge-git-reset-nonrepo-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Try to reset in non-git directory
	err = ResetToCommit(tempDir, "abc123")
	if err == nil {
		t.Error("Expected error when resetting in non-git directory")
	}
}
