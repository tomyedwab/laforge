Your goal is to implement a complex project mostly independently. Whenever you
have to make a decision, such as choosing a technical solution, designing a user
interface, or selecting a library, you should put forth a proposal in an
artifact and wait for feedback from me.

*EVERY TIME* YOU RUN, first look at currently in progress on in review tasks and
mark any completed that look like they are done! Do this EVERY TIME.

Every time I ask you to run you will follow these steps:
1. First, read README.md for project context.
2. Use the `latasks` CLI tool to get the next ready task to work on.
3. If the retrieved task has already been completed, or it has been reviewed and
   has no follow-up work in the log, MARK IT COMPLETE and move on to the next
   one.
4. Read any files in docs/ relevant to the task at hand.
5. Make a plan for making progress on the task and update the task log.
6. If the plan involves a decision or requires feedback, attach a review request
   to the task with links to relevant artifacts. Add a log message on the task
   documenting next steps to take once the review is accepted and stop working.
7. Otherwise, proceed with implementation work, make changes to the local
   codebase in /src as needed, keeping the task updated with log messages at
   regular intervals.
8. After making progress on or completing the task, update the task log with a
   summary of what was done and what work remains, and/or update the task
   status. If the task requires review, YOU MUST SEND A REVIEW REQUEST BEFORE
   STOPPING WORK.
9. Once the task is updated, clean up any temporary files (e.g., build
   artifacts, logs) and write a commit message in COMMIT.md.

## Task management

The `latasks` command-line tool is used to list, view, and update tasks. Tasks
are numbered starting at T1, going to T2, T3, etc. Tasks can contain other
tasks, for instance T1 can be a project containing sub-tasks T2 and T3. T1
should not be completed until all its subtasks are completed.

Available commands:
- `latasks next`: Retrieve the next task that is ready for work. Returns tasks
  in 'todo', 'in-progress', or 'in-review' status (with no pending reviews)
  where all upstream dependencies are completed.
- `latasks add <title> <parent_id?>`: Create a new task. If `parent_id` is
  specified, add the new task as a subtask. Returns the new task ID.
- `latasks view <task_id>`: View details of a specific task.
- `latasks update <task_id> <status>`: Update the status of a task.
- `latasks log <task_id> <message>`: Update the task log with a summary of what
  was done and what work remains.
- `latasks review <task_id> <message> <attachment>`: Send a review request and move the
  task to "in-review". This should be the last update to the task in the
  session. The optional attachment is a path to a file in the source repository,
  which could be a Markdown file, an image, a diagram, etc.
- `latasks list`: List all tasks.
- `latasks delete <task_id>`: Delete a task.

Task statuses:
- `todo`: The task has not yet been started.
- `in-progress`: The task is currently being worked on.
- `in-review`: The task has an active review and is waiting for human feedback.
- `completed`: The task has been completed.

## Task breakdown process

The type of a task can be conveyed simply by inserting tags into the task title.
Some examples:

[EPIC]: A larger project consisting of multiple sub-tasks.
[FEAT]: A new feature or enhancement.
[BUG]: A bug or issue that needs to be fixed.
[PLAN]: A task to break down a large scope of work into epics & features. Submit
  the task list as an artifact for review before creating the relevant tasks.
[ARCH]: A task to research possible architectural solutions for a problem and
  get them reviewed.
[DESIGN]: A task to come up with possible visual designs (UI wireframes for
  example) and get them reviewed.

The [PLAN] task type deserves special mention: When planning, the goal is always
to create a YAML file containing the desired new or updated tasks, and have it
reviewed. If the file matches the expected format, when approved the tasks will
automatically be updated.

Reviews should always be required for [PLAN], [ARCH], and [DESIGN] tasks. The
other tasks do not require review unless technical or design decisions were made
as part of the process.

## Artifacts

Artifacts are files attached to a review that can be reviewed by a human, and
therefore need to be human readable. Thus, the format needs to be one of the
following:

- YAML specification for tasks to create/update, as documented below
- Markdown document, for one-pagers and written architecture documents
- PlantUML diagram, for actor models, architectural diagrams (using NPlant), and UI mocks (using Salt)
- Mermaid diagram, for architecture diagrams
- Python pseudocode, for algorithmic descriptions

Artifacts may be stored in a `docs/artifacts` directory in the project root.
Once a review is completed, the artifact can be deleted, updated based on
feedback, or left as-is for future reference.

PLEASE KEEP THE REVIEW MESSAGE SHORT AND KEEP EACH ARTIFACT TO ONE PAGE IN
LENGTH, NO MORE!

## Task definition YAML

This is the format for task definitions:

```yaml
tasks:
  - id: 1
    title: "Implement user authentication system"
    description: "Create a complete authentication system with login, logout, and registration"
    acceptance_criteria: |
      - Users can register with email and password
      - Users can login with valid credentials
      - Users can logout
      - Password reset functionality works
      - Session management is secure
    upstream_dependency_id: null
    review_required: true
    parent_id: null
    status: "todo"

  - id: 2
    title: "Design database schema"
    description: "Create the database schema for user management"
    acceptance_criteria: |
      - Users table with required fields
      - Proper indexing for performance
      - Migration scripts are created
    upstream_dependency_id: null
    review_required: true
    parent_id: null
    status: "completed"

  - id: 3
    title: "Implement login endpoint"
    description: "Create REST API endpoint for user login"
    acceptance_criteria: |
      - POST /api/login endpoint exists
      - Returns JWT token on success
      - Proper error handling for invalid credentials
      - Rate limiting implemented
    upstream_dependency_id: 2  # Depends on database schema
    review_required: true
    parent_id: 1  # Child of authentication system
    status: "in-progress"
```

When a YAML document of this type is reviewed and approved, the tasks will
automatically be updated in the task database.

## Documentation

Documentation may be added in the `docs` directory in the project root. Whenever
a complex system is being implemented, keep implementation notes in a Markdown
file in the docs directory for that feature and link to the file in tasks and
code files. Similarly, link to relevant files ~~liberally~~ using their absolute
paths in the project in the documentation.
