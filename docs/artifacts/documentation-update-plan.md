# Documentation Update Plan for Step Database Features

## Overview
This document outlines the plan for updating LaForge documentation to include comprehensive information about the step database functionality that has been implemented.

## Current Documentation Status

### README.md
- Contains basic step execution flow but lacks step database details
- Missing information about step management commands
- No examples of step database usage

### Architecture Documentation (docs/specs/)
- `laforge.md` has basic step database mention but lacks details
- `latasks.md` focuses on task management, no step database integration
- Missing comprehensive step database schema documentation

### CLI Help Documentation
- Commands exist but need comprehensive documentation
- Missing usage examples and detailed explanations

## Required Updates

### 1. README.md Updates
**Location**: Lines 26-35 (step metadata section)
**Additions**:
- Step database purpose and capabilities
- Step management commands overview
- Step rollback functionality explanation
- Token usage tracking information

### 2. CLI Command Documentation
**Commands to document**:
- `laforge steps [project-id]` - List all steps
- `laforge step info [project-id] [step-id]` - Detailed step information  
- `laforge step rollback [project-id] [step-id]` - Rollback functionality

### 3. Architecture Documentation Updates
**docs/specs/laforge.md**:
- Expand step database section with full schema details
- Add step management command specifications
- Include step rollback architecture

**New documentation needed**:
- Step database schema reference
- Step lifecycle documentation
- Integration with task management system

### 4. API Documentation
- Step database REST API endpoints (if applicable)
- Step data model documentation
- Error handling specifications

### 5. Usage Examples
- Basic step command usage
- Step rollback scenarios
- Token usage analysis examples
- Integration with task workflow

## Implementation Plan

1. **Update README.md** - Add step database section
2. **Update CLI help text** - Enhance command descriptions
3. **Update architecture docs** - Expand specifications
4. **Create examples** - Add usage documentation
5. **Update API docs** - Document data models

## Files to Create/Update

### Update Existing:
- `/src/README.md` - Add step database section
- `/src/docs/specs/laforge.md` - Expand step database details
- `/src/cmd/laforge/main.go` - Enhance command help text

### Create New:
- `/src/docs/examples/step-commands.md` - Usage examples
- `/src/docs/specs/step-database.md` - Detailed schema documentation

## Acceptance Criteria
- [ ] README.md includes comprehensive step database information
- [ ] CLI help text provides detailed command usage
- [ ] Architecture documentation covers step database schema
- [ ] Usage examples are provided for all step commands
- [ ] API documentation includes step data models