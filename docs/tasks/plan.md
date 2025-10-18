# [PLAN] tasks and task planning

The goal for a planning task is to break down a project into a series of tasks,
and create a YAML file containing the updated or newly-created tasks to insert
into the task database.

Once the task breakdown process is complete and the task list is ready to be
approved, the yaml file should be submitted for review. You can stop work at
this point.

IT IS VERY IMPORTANT THAT BROKEN DOWN TASKS ARE APPROVED BEFORE STARTING
IMPLEMENTATION! DO NOT UNDER ANY CIRCUMSTANCES MOVE DIRECTLY FROM PLANNING TO
IMPLEMENTATION WITHOUT APPROVAL!

## Task breakdown process

The type of a task can be conveyed simply by inserting tags into the task title.
Some examples:

[EPIC]: A larger project consisting of multiple sub-tasks.
[FEAT]: A new feature or enhancement.
[BUG]: A bug or issue that needs to be fixed.
[PLAN]: A task to break down a large scope of work into epics & features. Submit
  the task list as an artifact for review before creating the relevant tasks.
[DOC]: A task to create new documentation or update documentation.
[ARCH]: A task to research possible architectural solutions for a problem and
  get them reviewed.
[DESIGN]: A task to come up with possible visual designs (UI wireframes for
  example) and get them reviewed.
[TEST]: A task to write comprehensive unit tests for a recently-implemented
  feature.

Reviews should always be required for [PLAN], [ARCH], [DOC], and [DESIGN] tasks.
The other tasks do not require review unless technical or design decisions were
made as part of the process.

## Review artifacts

The [PLAN] task type always has the same goal: to create a YAML file containing
the desired new or updated tasks, and have it reviewed. If the file matches the
expected format, when approved the tasks will automatically be updated once it
is reviewed.

## Task definition YAML

This is the format for task definitions:

```yaml
tasks:
  - id: 1
    title: "[EPIC] Implement user authentication system"
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
    title: "[ARCH] Design database schema"
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
    title: "[FEAT] Implement login endpoint"
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
