# README.md Update Plan for LaForge Dev Server & Web UI Documentation

## Current State Analysis

The README.md currently contains:
- Basic project overview and background
- High-level technical implementation details
- Step execution flow description
- Code layout section mentioning binaries

## Missing Documentation

### 1. CLI Tools Documentation
- **laforge**: Main orchestration tool (documented but incomplete)
- **laserve**: API server for web UI (missing)
- **latasks**: Task management CLI (missing)
- **latools**: Task utilities (missing)

### 2. Web UI Documentation
- Development server setup and usage
- Build and deployment instructions
- Environment configuration
- Feature overview

### 3. Development Workflow
- Complete setup instructions for new developers
- Running the development environment
- Testing procedures
- Integration between components

## Proposed README.md Structure

### 1. Quick Start Section
```markdown
## Quick Start

### Prerequisites
- Go 1.21+
- Node.js 18+
- Docker
- Git

### Installation
# Build all binaries
go build -o bin/ ./cmd/...

# Install dependencies for web UI
cd web-ui && npm install
```

### 2. CLI Tools Section
```markdown
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
```

### 3. Web UI Section
```markdown
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
```

### 4. Architecture Overview
```markdown
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
```

### 5. Complete Development Setup
```markdown
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
```

## Implementation Plan

1. **Update README.md** with the new sections above
2. **Add examples** for each CLI tool
3. **Document environment variables** and configuration
4. **Include architecture diagram** and data flow
5. **Add troubleshooting section** for common issues
6. **Update code layout section** with current structure

## Acceptance Criteria

- [ ] Complete CLI tool documentation with examples
- [ ] Web UI setup and development instructions
- [ ] Environment configuration documentation
- [ ] Architecture overview with diagrams
- [ ] Complete development workflow guide
- [ ] Troubleshooting section for common issues
- [ ] Updated code layout section reflecting current structure