# Step Rollback Design

## Overview
This document outlines the design for implementing step rollback functionality in LaForge, allowing users to revert to a previous step by deactivating subsequent steps and reverting git changes.

## Requirements

### Functional Requirements
1. **Step Deactivation**: Mark steps as inactive starting from a specified step ID
2. **Git Reversion**: Revert git repository to the state before the rollback target step
3. **Command Interface**: Provide `laforge step rollback [project-id] [step-id]` command
4. **Validation**: Ensure rollback target step exists and is valid
5. **Logging**: Track rollback operations for audit purposes

### Technical Requirements
1. **Database Operations**: Deactivate all steps after the target step ID
2. **Git Operations**: Reset repository to commit SHA before the target step
3. **Error Handling**: Comprehensive error handling with user-friendly messages
4. **Safety Checks**: Prevent rollback of already rolled-back steps

## Implementation Design

### Command Structure
```bash
laforge step rollback [project-id] [step-id]
```

### Database Schema Impact
- Uses existing `active` field in `steps` table
- No schema changes required

### Git Operations Required
1. **Get target step commit SHA**: Retrieve `commit_sha_before` from target step
2. **Reset repository**: Use `git reset --hard <commit_sha>` to revert changes
3. **Clean working directory**: Ensure clean state after rollback

### Database Operations Required
1. **Deactivate subsequent steps**: Set `active = FALSE` for all steps with ID >= target step ID
2. **Validate step exists**: Check target step exists before proceeding
3. **Audit logging**: Record rollback operation in step database

## Implementation Steps

### 1. Add Database Method
```go
// DeactivateStepsFromID deactivates all steps with ID >= given stepID
func (sdb *StepDatabase) DeactivateStepsFromID(stepID int) error
```

### 2. Add Git Operations
```go
// ResetToCommit resets the repository to the specified commit SHA
func ResetToCommit(repoDir string, commitSHA string) error
```

### 3. Implement Rollback Command
```go
// runStepRollback handles the step rollback command
func runStepRollback(cmd *cobra.Command, args []string) error
```

### 4. Add Command Registration
Add rollback subcommand to the step command group in main.go

## Error Handling

### Validation Errors
- Invalid project ID
- Invalid step ID format
- Target step not found
- Target step already rolled back

### Operational Errors
- Database connection failures
- Git operation failures
- Permission issues

### User Feedback
- Clear error messages with suggestions
- Progress indicators during rollback
- Success confirmation with details

## Safety Considerations

### Pre-rollback Checks
1. Verify target step exists and is active
2. Ensure git repository is in clean state
3. Confirm rollback target is not the current HEAD

### Post-rollback Actions
1. Verify repository state matches target commit
2. Confirm step deactivation was successful
3. Log rollback operation for audit trail

## Testing Strategy

### Unit Tests
- Database deactivation methods
- Git reset operations
- Command validation logic

### Integration Tests
- Full rollback workflow
- Error handling scenarios
- Multi-step rollback scenarios

### Edge Cases
- Rollback to step 1 (initial state)
- Rollback with no subsequent steps
- Rollback after partial step execution

## Future Enhancements

### Advanced Features
- Selective rollback (specific files/directories)
- Rollback preview (dry-run mode)
- Rollback history tracking
- Automatic backup creation

### Performance Optimizations
- Batch database operations
- Parallel git operations
- Cached step metadata

## Implementation Priority

1. **Core functionality** (High)
   - Database deactivation method
   - Git reset operations
   - Basic rollback command

2. **Validation and safety** (High)
   - Pre-rollback checks
   - Error handling
   - User confirmation

3. **Logging and auditing** (Medium)
   - Rollback operation logging
   - Audit trail maintenance

4. **Advanced features** (Low)
   - Selective rollback
   - Preview mode
   - Performance optimizations