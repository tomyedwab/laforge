# LaForge: A long-running collaborative coding agent

## Quick Start

### Prerequisites
- Go 1.21+
- Node.js 18+
- Docker
- Git

### Installation
```bash
# Build all binaries
go build -o bin/ ./cmd/...

# Install dependencies for web UI
cd web-ui && npm install
```

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

### Step Database

Each step is recorded in a SQLite database that tracks:
- **Step ID**: Sequential integer starting at 1
- **Active Status**: Whether the step is active (can be deactivated during rollback)
- **Parent Step ID**: Links to previous step for maintaining execution history
- **Commit SHAs**: Git commit before and after step execution
- **Agent Configuration**: Complete agent settings used for the step
- **Timing Information**: Start time, end time, and duration in milliseconds
- **Token Usage**: Prompt tokens, completion tokens, total tokens, and cost
- **Exit Code**: Container exit status for debugging

### Step Management Commands

LaForge provides several commands for managing step history:

```bash
# List all steps for a project
laforge steps [project-id]

# Show detailed information about a specific step
laforge step info [project-id] [step-id]

# Rollback to a previous step (deactivates subsequent steps and reverts git changes)
laforge step rollback [project-id] [step-id]
```

### Step Rollback Functionality

The rollback feature allows you to revert your project to any previous step:
1. **Step Deactivation**: All steps after the target step are marked as inactive
2. **Git Repository Reset**: The repository is reset to the commit before the target step
3. **Safety Confirmation**: User confirmation is required before performing rollback

This provides a safe way to explore different approaches and easily revert changes if needed.

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

## Web UI

LaForge includes a modern web interface built with Preact and TypeScript.

### Development Server

1. **Install dependencies:**
   ```bash
   cd web-ui
   npm install
   ```

2. **Configure environment:**
   ```bash
   cp .env.example .env.local
   # Edit .env.local with your configuration
   ```

3. **Start development server:**
   ```bash
   npm run dev
   ```

The web UI will be available at `http://localhost:3000`.

### Environment Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `VITE_API_BASE_URL` | API base URL | `http://localhost:8080/api/v1` |
| `VITE_WS_URL` | WebSocket URL | `ws://localhost:8080/api/v1` |
| `VITE_AUTH_TOKEN_KEY` | LocalStorage key for auth token | `laforge_auth_token` |
| `VITE_PROJECT_ID` | Default project ID | `laforge` |
| `VITE_ENV` | Environment | `development` |

### Features

- **Task Management**: View, create, update, and delete tasks
- **Real-time Updates**: Live task status updates via WebSocket
- **Review Workflow**: Submit and review task artifacts
- **Step History**: Visualize step execution history
- **Responsive Design**: Works on desktop and mobile devices
- **Dark Mode**: Automatic theme switching

### Build for Production

```bash
cd web-ui
npm run build
```

The built files will be in the `dist/` directory, ready to be served by any static file server.

### Development Scripts

- `npm run dev` - Start development server
- `npm run build` - Build for production
- `npm run preview` - Preview production build
- `npm run lint` - Run ESLint
- `npm run lint:fix` - Fix ESLint issues
- `npm run format` - Format code with Prettier
- `npm run test` - Run tests
- `npm run test:ui` - Run tests with UI
- `npm run test:coverage` - Run tests with coverage

## Code layout

## Architecture

### Component Interaction

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Web UI        │    │   laserve       │    │   laforge       │
│   (Preact/TS)   │◄──►│   (API Server)  │◄──►│   (Orchestrator)│
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌─────────────────┐
                       │   SQLite        │
                       │   Database      │
                       └─────────────────┘
```

### Data Flow

1. **Web UI** communicates with **laserve** via REST API and WebSocket
2. **laserve** manages SQLite databases for tasks and project data
3. **laforge** orchestrates agent execution and updates databases
4. **latasks** provides direct CLI access to task management

### Database Schema

- **Tasks**: Hierarchical task management with dependencies
- **Steps**: Execution history with timing and token usage
- **Reviews**: Review workflow with attachments and feedback
- **Logs**: Task activity logging

LaForge consists of several binary entrypoints in the cmd/ folder, which use
shared modules in tasks/, projects/, etc.

The binaries are:
- laforge: The main executable that handles the main loop and provides the API server (used by the web UI) as well as the specialized tools for managing tasks and artifacts.
- latasks: Tool exposed to the agent container for managing tasks.
- latools: Tools meant for debugging and testing in the host environment.

## Complete Development Setup

### 1. Build All Components

```bash
# Build all Go binaries
go build -o bin/ ./cmd/...

# Install web UI dependencies
cd web-ui && npm install
```

### 2. Initialize Project

```bash
# Create a new LaForge project
./bin/laforge init my-project

# Start the API server
./bin/laserve --host 0.0.0.0 --port 8080 --jwt-secret dev-secret --env development
```

### 3. Start Web UI Development

```bash
cd web-ui
npm run dev
```

### 4. Run LaForge Steps

```bash
# In another terminal, run LaForge steps
./bin/laforge step my-project
```

### 5. Access the Web Interface

- Web UI: http://localhost:3000
- API Server: http://localhost:8080
- API Documentation: Available via laserve health endpoint

## CLI Tools

LaForge provides several command-line tools:

### laforge - Main Orchestration Tool
Manages projects, runs steps, and handles the core LaForge workflow.

**Commands:**
- `laforge init <project-id>` - Initialize a new project
- `laforge step <project-id>` - Run a single step
- `laforge steps <project-id>` - List all steps for a project
- `laforge step info <project-id> <step-id>` - Show detailed step information
- `laforge step rollback <project-id> <step-id>` - Rollback to a previous step

**Examples:**
```bash
# Initialize a new project
laforge init my-project --name "My Project" --description "Project description"

# Run a single step
laforge step my-project

# List all steps
laforge steps my-project

# Get detailed step information
laforge step info my-project S1
```

### laserve - API Server
Provides REST API and WebSocket server for the web UI.

**Usage:**
```bash
laserve --host 0.0.0.0 --port 8080 --jwt-secret your-secret-key
```

**Options:**
- `--host`: Server host (default: 0.0.0.0)
- `--port`: Server port (default: 8080)
- `--jwt-secret`: JWT secret for authentication (required)
- `--env`: Environment (development, staging, production)

### latasks - Task Management CLI
Manage tasks directly from the command line.

**Commands:**
- `latasks next` - Get the next task ready for work
- `latasks add <title> [parent-id]` - Create a new task
- `latasks view <task-id>` - View task details
- `latasks update <task-id> <status>` - Update task status
- `latasks log <task-id> <message>` - Add log entry
- `latasks review <task-id> <message> [attachment]` - Create review request
- `latasks list` - List all tasks
- `latasks delete <task-id>` - Delete a task

**Examples:**
```bash
# Create a new task
latasks add "Implement authentication" --description "Add user login functionality"

# Update task status
latasks update T1 in-progress

# Add log entry
latasks log T1 "Started implementation of auth endpoints"

# Create review request
latasks review T1 "Please review the authentication design" docs/auth-design.md
```

### latools - Task Utilities
Utility tools for task management.

**Commands:**
- `latools import <yaml-file>` - Import tasks from YAML file
- `latools review` - Interactively review pending reviews

**Examples:**
```bash
# Import tasks from YAML
latools import tasks.yml

# Review pending reviews interactively
latools review
```
