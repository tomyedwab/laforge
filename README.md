# LaForge: A long-running collaborative coding agent

## Background

LaForge is an experimental coding agent that can run for long periods of time
semi-autonomously. The emphasis is on giving the agent explicit tools for
getting frequent feedback on its work artifacts and prompting the agent to use
them regularly to allow the work to be monitored by a human.

While the agent can be a script or CLI tool running in a terminal, the
monitoring UI will show a structured view of the agent's task progress and
proactively notify the human user when the agent has clarification questions or
artifacts to review. Artifacts can be text, images, screenshots, or diagrams,
and the review feedback consists of an Accept or Reject action along with any
suggestions and feedback. These artifacts can include high-level plans,
architectural decisions, technical specifications, UI mockups, and short code
snippets, so that the agent is always making positive progress.

## Project management

LaForge stores project metadata in the user's home directory at
~/.laforge/projects/<name>, which is created by the `laforge init` command and
is not committed to source control. This allows the agent to manage the full
history of multiple projects without cluttering the repository with metadata.

Every call to the LaForge implementation loop is a numbered "step", starting at
S1 and increasing monotonically. Since each step is associated with a commit in
the repository, it is easy to revert to a previous state if needed. The step
metadata includes a snapshot of the task state sqlite database and any container
logs from the step. A file in the project root specifies the current committed
step as well as any currently running steps. The metadata also includes the log
of steps tracking time and tokens spent on each step.

If LaForge chooses to run multiple steps in parallel, it will have to create
separate branches for each step and run a separate step to merge them together.

## Technical implementation

The technical process for enabling a long-running agent is to prompt the agent
to break up its work up into a series of tasks and taking repeated small steps
forward on the next task. The coding agent can be any LLM-based agent that runs on the
command line, such as Claude Code or Opencode, with the LaForge host process
invoking it and responding to its outputs.

Each step runs as follows:

1. On the host machine in the host process, a temporary git worktree is created
   in a new directory pointing at a newly-created branch off main.
2. Also on the host machine, a copy of the task database (a simple SQLite
   database) is made.
3. The agent is invoked inside a Docker container with the worktree directory
   and task database mounted as volumes. The agent is given minimal
   instructions and must rely on general instructions/documentation in the
   repository along with the tasks database to decide what work to start.
4. The agent is given minimal command-line tools for interacting with the source
   code, building and running the application, and updating the task database.
   Some of these execute directly and some run in other containers or interact
   with the host, but the container is a sandboxed environment and nothing
   should be written outside of it.
5. The agent can complete tasks by writing code or documentation, updating its
   own instructions, or creating artifacts and requesting feedback on them.
   In any of these cases it will stop execution and exit the container.
6. The host process captures changes any changes to the source code, commits
   them to the temporary branch, and pushes the branch to main.
7. The host process captures changes to tasks and updates the monitor UI,
   sending notifications for any requested feedback.
8. The host process stores any logs and timing information for the step into its
   running log.
9. The temporary copies are cleaned up and the worktree is deleted. The process
   restarts from step 1 until all tasks are completed or the process is
   interrupted.

LaForge is implemented in Go, including the main executable that handles the
main loop and provides the API server (used by the web UI) as well as the
specialized tools for managing tasks and artifacts.

The LaForge UI is a web application that provides a user-friendly
interface for interacting with the LaForge agent. The most important features of
this UI are:
- Viewing task progress in real-time
- Receiving review request notifications (via push notifications)
- Viewing review artifacts and sending feedback
- Viewing agent logs and interrupting the agent if it is misbehaving
- [Stretch goal] Tracking token usage and cost

## Code layout

LaForge consists of several binary entrypoints in the cmd/ folder, which use
shared modules in tasks/, projects/, etc.

The binaries are:
- laforge: The main executable that handles the main loop and provides the API server (used by the web UI) as well as the specialized tools for managing tasks and artifacts.
- latasks: Tool exposed to the agent container for managing tasks.
