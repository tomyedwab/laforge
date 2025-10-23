Your goal is to implement a complex project mostly independently. Whenever you
have to make a decision, such as choosing a technical solution, designing a user
interface, or selecting a library, you should put forth a proposal in an
artifact and wait for feedback from me.

*EVERY TIME* YOU RUN, first look at currently in progress on in review tasks and
mark any completed that look like they are done! Do this EVERY TIME.

Every time I ask you to run you will follow these steps:
1. First, read README.md for project context.
2. Use the `latasks next` CLI tool to get the next ready task to work on and
   lease it with `latasks lease`.
3. IF THERE ARE NO TASKS READY TO WORK ON, STOP IMMEDIATELY.
4. If the retrieved task has already been completed, or it has been reviewed and
   has no follow-up work in the log, MARK IT COMPLETE and move on to the next
   one.
5. Read any files in docs/ relevant to the task at hand.
6. Make a plan for making progress on the task and update the task log.
7. If the plan involves a decision or requires feedback, attach a review request
   to the task with links to relevant artifacts. Add a log message on the task
   documenting next steps to take once the review is accepted and STOP WORKING
   IMMEDIATELY - DO NOT START IMPLEMENTATION UNTIL A PLAN HAS BEEN REVIEWED!
8. Otherwise, proceed with implementation work, make changes to the local
   codebase in /src as needed, keeping the task updated with log messages at
   regular intervals.
9. After making progress on or completing the task, update the task log with a
   summary of what was done and what work remains, and/or update the task
   status. If the task requires review, YOU MUST SEND A REVIEW REQUEST BEFORE
   STOPPING WORK.
10. Once the task is updated, clean up any temporary files (e.g., build
    artifacts, logs) and write a commit message in COMMIT.md.

## Task management

The `latasks` command-line tool is used to list, view, and update tasks. Tasks
are numbered starting at T1, going to T2, T3, etc. Tasks can contain other
tasks, for instance T1 can be a project containing sub-tasks T2 and T3. T1
should not be completed until all its subtasks are completed.

## Available commands

The following can be used to read tasks from the database at any time:
- `latasks next`: Retrieve the next unleased task that is ready for work.
  Returns tasks in 'todo', 'in-progress', or 'in-review' status (with no pending
  reviews) where all upstream dependencies are completed.
- `latasks view <task_id>`: View details of a specific task.
- `latasks list`: List all tasks.

To take ownership of a task, use:
- `latasks lease <task_id>`: Leases a task for the duration of this session.

Once a task is leased, you can use these tools to queue updates:
- `latasks update <task_id> <status>`: Update the status of a task.
- `latasks log <task_id> <message>`: Update the task log with a summary of what
  was done and what work remains.
- `latasks review <task_id> <message> <attachment>`: Send a review request and move the
  task to "in-review". This should be the last update to the task in the
  session. The optional attachment is a path to a file in the source repository,
  which could be a Markdown file, an image, a diagram, etc.

NOTE: Updates are not visible in view/list while the task is leased!

Task statuses:
- `todo`: The task has not yet been started.
- `in-progress`: The task is currently being worked on.
- `in-review`: The task has an active review and is waiting for human feedback.
- `completed`: The task has been completed.

Task types:
- [EPIC]: A larger project consisting of multiple sub-tasks
- [PLAN]: An epic or complex task needs to be broken down into subtasks (read `doc/tasks/plan.md` before starting)
- [FEAT]: A new feature to be implemented
- [BUG]: A bug to be fixed
- [ARCH]: An architectural decision to be made
- [DOC]: Documentation needs updating
- [DESIGN]: Design decisions to be made
- [TEST]: Test cases to be written

## Artifacts

Artifacts are files attached to a review that can be reviewed by a human, and
therefore need to be human readable. Thus, the format needs to be one of the
following:

- YAML specification for tasks to create/update, as documented in `doc/tasks/plan.md`.
- Markdown document, for one-pagers and written architecture documents
- PlantUML diagram, for actor models, architectural diagrams (using NPlant), and UI mocks (using Salt)
- Mermaid diagram, for architecture diagrams
- Python pseudocode, for algorithmic descriptions

Artifacts may be stored in a `docs/artifacts` directory in the project root.
Once a review is completed, the artifact can be deleted, updated based on
feedback, or left as-is for future reference.

PLEASE KEEP THE REVIEW MESSAGE SHORT AND KEEP EACH ARTIFACT TO ONE PAGE IN
LENGTH, NO MORE!

## Documentation

Documentation may be added in the `docs` directory in the project root. Whenever
a complex system is being implemented, keep implementation notes in a Markdown
file in the docs directory for that feature and link to the file in tasks and
code files. Similarly, link to relevant files ~~liberally~~ using their absolute
paths in the project in the documentation.
