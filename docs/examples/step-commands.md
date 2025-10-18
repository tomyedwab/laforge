# Step Command Usage Examples

This document provides comprehensive examples of using LaForge step management commands.

## Prerequisites

Before using step commands, ensure you have:
- A LaForge project initialized (`laforge init [project-id]`)
- At least one step executed (`laforge step [project-id]`)
- Git repository with commits for step tracking

## Listing Steps

### Basic Step Listing

List all steps for a project:

```bash
laforge steps my-project
```

Example output:
```
Step ID: S1
  Status: active
  Duration: 2m 45s
  Commits: abc123 -> def456
  Exit Code: 0
  Tokens: 1,234 total

Step ID: S2
  Status: active
  Duration: 1m 23s
  Commits: def456 -> ghi789
  Exit Code: 0
  Tokens: 2,567 total

Step ID: S3
  Status: active
  Duration: 3m 12s
  Commits: ghi789 -> jkl012
  Exit Code: 1
  Tokens: 3,891 total
```

### Filtering Step Output

Use standard Unix tools to filter and process step information:

```bash
# Show only the most recent 5 steps
laforge steps my-project | head -25

# Find steps with errors (non-zero exit codes)
laforge steps my-project | grep -A 5 "Exit Code: [1-9]"

# Extract step IDs for scripting
laforge steps my-project | grep "Step ID:" | awk '{print $3}'
```

## Detailed Step Information

### Viewing Step Details

Get comprehensive information about a specific step:

```bash
laforge step info my-project S1
```

Example output:
```
Step Information for S1
========================
Basic Info:
  Step ID: S1
  Status: active
  Project: my-project
  Created: 2025-10-18 08:30:15

Execution Timeline:
  Started: 2025-10-18 08:30:15
  Ended: 2025-10-18 08:32:45
  Duration: 2m 30s (150,000ms)

Git Information:
  Commit Before: abc123def456
  Commit After: def456ghi789
  Files Changed: 12
  Insertions: 245
  Deletions: 89

Agent Configuration:
  Model: claude-3.5-sonnet
  Temperature: 0.7
  Max Tokens: 4000
  Tools: bash, read, write, edit
  System Prompt: Standard LaForge agent configuration

Token Usage:
  Prompt Tokens: 1,234
  Completion Tokens: 567
  Total Tokens: 1,801
  Estimated Cost: $0.023

Execution Results:
  Exit Code: 0
  Status: Success
  Container ID: laforge-step-S1-abc123
```

### Comparing Steps

Compare configurations between different steps:

```bash
# Get agent config for step S1
laforge step info my-project S1 | grep -A 10 "Agent Configuration"

# Get agent config for step S2
laforge step info my-project S2 | grep -A 10 "Agent Configuration"

# Compare token usage
for step in S1 S2 S3; do
  echo "=== $step ==="
  laforge step info my-project $step | grep -A 4 "Token Usage"
done
```

## Step Rollback

### Basic Rollback

Rollback to a previous step:

```bash
laforge step rollback my-project S2
```

Example interaction:
```
WARNING: This will permanently deactivate steps S3 and later and reset your git repository.

Steps to be deactivated:
  - S3 (active, 3m 12s, exit code: 1)
  - S4 (active, 1m 45s, exit code: 0)

Git changes:
  - Current HEAD: jkl012 (after S4)
  - Will reset to: def456 (before S3)

Type 'rollback' to confirm: rollback

Rolling back to step S2...
✓ Deactivated 2 steps (S3, S4)
✓ Git repository reset to def456
✓ Rollback completed successfully
```

### Rollback Safety Features

The rollback command includes several safety measures:

1. **User Confirmation**: Requires typing "rollback" to confirm
2. **Git State Check**: Ensures no uncommitted changes exist
3. **Step Impact Preview**: Shows which steps will be deactivated
4. **Git Impact Preview**: Shows the git reset that will occur

### Common Rollback Scenarios

#### Scenario 1: Reverting Problematic Changes

Your agent made changes that broke the build:

```bash
# Check recent steps
laforge steps my-project

# Identify the problematic step (S5 with exit code 1)
laforge step info my-project S5

# Rollback to the last working step (S4)
laforge step rollback my-project S4

# Continue with a new step
laforge step my-project
```

#### Scenario 2: Exploring Alternative Approaches

You want to try a different approach to solving a problem:

```bash
# Current state after S7
laforge step info my-project S7

# Rollback to S5 to try a different approach
laforge step rollback my-project S5

# Run new steps with different agent configuration
laforge step my-project --agent-config=alternative-config
```

#### Scenario 3: Recovery from Agent Errors

The agent encountered an error and you want to retry:

```bash
# Check what went wrong
laforge step info my-project S9

# Rollback to before the error
laforge step rollback my-project S8

# Retry with adjusted parameters
laforge step my-project --timeout=30m
```

## Token Usage Analysis

### Monitoring Token Consumption

Track token usage across steps:

```bash
# Get token usage for all steps
for step in $(laforge steps my-project | grep "Step ID:" | awk '{print $3}'); do
  usage=$(laforge step info my-project $step | grep "Total Tokens:" | awk '{print $3}')
  echo "$step: $usage tokens"
done
```

### Cost Estimation

Estimate costs based on token usage:

```bash
# Calculate total cost for all steps
total_cost=0
for step in $(laforge steps my-project | grep "Step ID:" | awk '{print $3}'); do
  cost=$(laforge step info my-project $step | grep "Estimated Cost:" | awk '{print $3}')
  total_cost=$(echo "$total_cost + $cost" | bc)
done
echo "Total cost: \$$total_cost"
```

## Integration with Task Management

### Correlating Steps with Tasks

While steps and tasks are independent, you can correlate them:

```bash
# Get the current step ID from the most recent step
CURRENT_STEP=$(laforge steps my-project | grep "Step ID:" | tail -1 | awk '{print $3}')

# Get task status
latasks list

# Add a log entry referencing the current step
latasks log T42 "Completed step $CURRENT_STEP with agent configuration updates"
```

### Workflow Example

Complete workflow combining steps and tasks:

```bash
# 1. Initialize project
laforge init my-project

# 2. Create task for feature implementation
latasks add "Implement user authentication" T1

# 3. Run step to work on the task
laforge step my-project

# 4. Check step results
laforge step info my-project S1

# 5. If successful, update task status
latasks update T2 completed

# 6. If issues found, rollback and retry
laforge step rollback my-project S1
laforge step my-project --agent-config=debug-config
```

## Troubleshooting

### Common Issues

**Issue**: "No steps found for project"
**Solution**: Ensure you have run at least one step:
```bash
laforge step my-project
```

**Issue**: "Step not found: S99"
**Solution**: Check available steps:
```bash
laforge steps my-project
```

**Issue**: "Git repository has uncommitted changes"
**Solution**: Commit or stash changes before rollback:
```bash
git add .
git commit -m "Work in progress"
# or
git stash
```

**Issue**: "Cannot rollback to inactive step"
**Solution**: Choose an active step:
```bash
laforge steps my-project | grep "Status: active"
```

### Getting Help

For more information about step commands:

```bash
laforge step --help
laforge steps --help
laforge step info --help
laforge step rollback --help
```