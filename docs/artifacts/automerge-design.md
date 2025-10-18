# Automerge Step Branch Design

## Overview
Implement automatic merging of step branches back to main branch after successful step completion, with proper error handling and cleanup.

## Current State
- Step execution creates temporary worktree with branch `step-S{ID}`
- Changes are committed to the step branch
- Branch cleanup is disabled (TODO comment in `RemoveWorktree`)
- No merge back to main occurs

## Proposed Implementation

### 1. Git Package Enhancements

#### New Functions
```go
// MergeBranch merges source branch into target branch
func MergeBranch(repoDir string, sourceBranch string, targetBranch string, message string) error

// DeleteBranch deletes a branch if it exists and is not current
func DeleteBranch(repoDir string, branchName string) error

// GetCurrentBranch returns the currently checked out branch
func GetCurrentBranch(repoDir string) (string, error)

// SwitchBranch switches to the specified branch
func SwitchBranch(repoDir string, branchName string) error

// BranchExists checks if a branch exists
func BranchExists(repoDir string, branchName string) (bool, error)
```

### 2. Step Execution Flow Changes

#### Modified `runStep` function:
1. After successful commit to step branch:
   - Switch to main branch
   - Merge step branch into main
   - If merge successful: delete step branch
   - If merge fails: keep step branch for manual resolution

#### Enhanced `RemoveWorktree` function:
- Remove the conditional branch deletion logic
- Always attempt branch cleanup after worktree removal
- Add merge functionality before branch deletion

### 3. Error Handling

#### Merge Conflict Resolution
- Detect merge conflicts during automerge
- Provide clear error messages with resolution steps
- Keep step branch intact for manual merge resolution
- Log detailed merge status information

#### Fallback Behavior
- If automerge fails, step branch remains
- User can manually merge using standard git commands
- Step is still marked as completed in database
- Clear notification of manual merge requirement

### 4. Implementation Steps

1. **Git Functions**: Implement merge and branch management functions
2. **Step Integration**: Modify `runStep` to perform automerge
3. **Cleanup Logic**: Update `RemoveWorktree` to handle post-merge cleanup
4. **Error Handling**: Add comprehensive merge conflict handling
5. **Testing**: Create integration tests for automerge scenarios

### 5. Safety Considerations

- Only merge if step completed successfully (exit code 0)
- Verify no uncommitted changes on main branch before merge
- Preserve step branch if merge fails for manual resolution
- Clear logging of all merge operations
- User notification of merge success/failure

### 6. Future Enhancements

- Configurable merge strategies (fast-forward vs. merge commit)
- Merge conflict resolution assistance
- Automatic merge conflict detection and reporting
- Integration with step rollback functionality