## Main agent harness

The `laforge` command is meant to be run on the host machine and provides the
main loop and API server for the agent container. It manages projects and passes
tasks to the agent containers it spawns, capturing logs, validating output and
passing review requests to the user via the API. See README.md for more
information.

The command line arguments for `laforge` are as follows:
```
laforge [options] [command]

Options:
  -h, --help      Show help information
  -v, --version   Show version information

Commands:
  init            Initialize a new project in ~/.laforge/projects/<id>
  step            Run a single step for the given project ID and exit
  run             Run all projects and API server
```

## Step database

Each step that is run using either the "step" or "run" command is recorded
locally in the project directory in a SQLite database. This database is accessed
only by the laforge CLI and is not exposed to the Docker containers running the
agent in each step.

### Step Database Schema

The step database uses the following schema:

```sql
CREATE TABLE steps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    active BOOLEAN DEFAULT TRUE,
    parent_step_id INTEGER,
    commit_sha_before TEXT NOT NULL,
    commit_sha_after TEXT,
    agent_config_json TEXT NOT NULL,
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP,
    duration_ms INTEGER,
    token_usage_json TEXT NOT NULL DEFAULT '{}',
    exit_code INTEGER,
    project_id TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_step_id) REFERENCES steps(id)
);

CREATE INDEX idx_steps_project_id ON steps(project_id);
CREATE INDEX idx_steps_active ON steps(active);
CREATE INDEX idx_steps_parent_step_id ON steps(parent_step_id);
CREATE INDEX idx_steps_created_at ON steps(created_at);
```

### Step Data Model

Each step stores the following information:

**Core Information:**
- `id`: Sequential integer starting at 1 (used for step identification)
- `active`: Boolean flag to identify steps that have been rolled back
- `parent_step_id`: Links to the most recent active step preceding this one
- `project_id`: The project this step belongs to

**Git Integration:**
- `commit_sha_before`: Git commit SHA before step execution
- `commit_sha_after`: Git commit SHA after step completion

**Agent Configuration:**
- `agent_config_json`: Serialized agent configuration including model, temperature, tools, and metadata

**Execution Metadata:**
- `start_time`: When the step began execution
- `end_time`: When the step completed (NULL if still running)
- `duration_ms`: Total execution time in milliseconds
- `exit_code`: Container exit status (NULL if still running)

**Token Usage Tracking:**
- `token_usage_json`: Serialized token usage statistics including prompt tokens, completion tokens, total tokens, and estimated cost

### Step Management Commands

The following commands are available for step management:

```bash
# List all steps for a project with summary information
laforge steps [project-id]

# Show detailed information about a specific step
laforge step info [project-id] [step-id]

# Rollback to a previous step (deactivates subsequent steps and reverts git)
laforge step rollback [project-id] [step-id]
```

### Step Rollback Functionality

The rollback feature provides a safe way to revert to previous project states:

1. **Step Deactivation**: All steps with ID >= target step are marked as inactive
2. **Git Repository Reset**: Repository is reset to the commit before the target step
3. **Safety Measures**: User confirmation required, clean git state enforced

### Integration with Task Management

The step database works alongside the task management system:
- Step execution is independent of task status
- Task databases are copied for each step execution
- Step rollback does not affect task status
- Both systems provide complementary project history tracking

The step ID is used when creating the temporary worktree, state, and logs directories.
