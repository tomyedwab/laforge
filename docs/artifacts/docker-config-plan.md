# Docker Configuration Enhancement Plan

## Overview
Based on the review feedback for T3, we need to make Docker commands much more configurable by storing image, volume mounts, environment variables, etc. under config names in `~/.laforge/projects/<id>/agents.yml` and passing the config name to the step command.

## Current State
- Docker configuration is hardcoded in the step command
- Only `--agent-image` and `--timeout` flags are configurable
- Volume mounts are fixed: workspace and data directory
- Environment variables are limited to basic LaForge variables

## Proposed Solution

### 1. Create agents.yml Configuration File
Store in `~/.laforge/projects/<id>/agents.yml` with the following structure:

```yaml
agents:
  default:
    image: laforge-agent:latest
    description: Default agent configuration
    volumes:
      - /workspace:/workspace
      - /data:/data
    environment:
      LAFORGE_PROJECT_ID: "${LAFORGE_PROJECT_ID}"
      LAFORGE_STEP: "true"
      CUSTOM_VAR: "custom_value"
    memory_limit: 1g
    cpu_shares: 1024
    timeout: 30m
    auto_remove: true
    
  opencode:
    image: opencode:latest
    description: Opencode agent with enhanced tools
    volumes:
      - /workspace:/workspace
      - /data:/data
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      LAFORGE_PROJECT_ID: "${LAFORGE_PROJECT_ID}"
      LAFORGE_STEP: "true"
      AGENT_TYPE: "opencode"
    memory_limit: 2g
    cpu_shares: 2048
    timeout: 1h
    auto_remove: true
    
  claude:
    image: claude-code:latest
    description: Claude Code agent
    volumes:
      - /workspace:/workspace
      - /data:/data
    environment:
      LAFORGE_PROJECT_ID: "${LAFORGE_PROJECT_ID}"
      LAFORGE_STEP: "true"
      ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
    memory_limit: 512m
    cpu_shares: 512
    timeout: 45m
    auto_remove: true
```

### 2. Update Step Command Interface
Change the step command to accept a config name:

```bash
laforge step <project-id> [--agent-config <config-name>] [--timeout <duration>]
```

- `--agent-config`: Specify which agent configuration to use (default: "default")
- `--timeout`: Override timeout from config (optional)

### 3. Implementation Tasks

#### T15: Create Agent Configuration Models
- Define Go structs for agent configuration
- Create YAML marshaling/unmarshaling functions
- Add validation for configuration parameters

#### T16: Implement agents.yml File Management
- Create functions to read/write agents.yml files
- Add default configuration generation
- Implement configuration validation

#### T17: Update Project Creation (init command)
- Generate default agents.yml during project initialization
- Allow custom agent configurations via flags or config file

#### T18: Refactor Docker Client to Use Configuration
- Update Docker client to accept agent configuration
- Implement volume mount parsing and validation
- Handle environment variable substitution

#### T19: Update Step Command Implementation
- Modify step command to load agent configuration
- Pass configuration to Docker client
- Handle configuration overrides (timeout, etc.)

#### T20: Add Configuration Management Commands
- Add commands to list, view, and update agent configurations
- Implement configuration validation
- Add ability to add/remove configurations

#### T21: Update Integration Tests
- Modify existing tests to use new configuration system
- Add tests for configuration loading and validation
- Test multiple agent configurations

#### T22: Update Documentation
- Document the agents.yml file format
- Update command reference
- Add configuration examples

### 4. Backward Compatibility
- Maintain support for existing `--agent-image` flag as fallback
- Provide migration path for existing projects
- Ensure default configuration matches current behavior

### 5. Error Handling
- Validate configuration on load
- Provide clear error messages for invalid configurations
- Graceful fallback to default configuration

### 6. Security Considerations
- Validate volume mount paths to prevent directory traversal
- Sanitize environment variable values
- Restrict sensitive configuration options

## Acceptance Criteria
- [ ] agents.yml file is created during project initialization
- [ ] Step command accepts `--agent-config` parameter
- [ ] Multiple agent configurations can be defined per project
- [ ] Configuration includes image, volumes, environment, resources
- [ ] Environment variable substitution works correctly
- [ ] Existing functionality remains intact
- [ ] Integration tests pass with new configuration system
- [ ] Documentation is updated

## Implementation Priority
1. T15 (Agent Configuration Models) - High
2. T16 (agents.yml File Management) - High  
3. T18 (Docker Client Refactoring) - High
4. T19 (Step Command Update) - High
5. T17 (Project Creation Update) - Medium
6. T20 (Configuration Management Commands) - Medium
7. T21 (Integration Tests) - Medium
8. T22 (Documentation) - Low