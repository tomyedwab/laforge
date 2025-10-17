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

A step stores the following information:
- The step ID (incrementing integer starting at 1)
- Active flag (boolean) to identify steps that have been rolled back
- The parent step ID (most recent active ID preceding this step)
- The commit shas before and after the step
- The agent configuration used to run the step
- Metadata for tracking start/end times and token usage

The step ID is used when creating the temporary worktree, state, and logs directories.
