## Task management

The `latasks` command-line tool is used to list, view, and update tasks. Tasks
are numbered starting at T1, going to T2, T3, etc. Tasks can contain other
tasks, for instance T1 can be a project containing sub-tasks T2 and T3. T1
should not be completed until all its subtasks are completed.

Available commands:
- `latasks next`: Retrieve the next task from the "upcoming work" queue.
- `latasks add <title> <parent_id?>`: Create a new task. If `parent_id` is
  specified, add the new task as a subtask. Returns the new task ID.
- `latasks queue <task_id>`: Add the task to the "upcoming work" queue.
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

## Implementation

`latasks` is written in Go which is built and added to the agent container in
/bin. The entrypoint is cmd/latasks/main.go and the logic is in a new package
tasks/ in this repository.

At runtime the tasks are stored in a SQLite database, which is assumed to
be mounted in the container as /state/tasks.db. The database schema is as
follows:

```sql
CREATE TABLE tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    parent_id INTEGER,
    status TEXT NOT NULL DEFAULT 'todo',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (parent_id) REFERENCES tasks(id) ON DELETE CASCADE,
    CHECK (status IN ('todo', 'in-progress', 'in-review', 'completed'))
);
```

```sql
CREATE TABLE task_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);
```

```sql
CREATE TABLE task_reviews (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    message TEXT NOT NULL,
    attachment TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    feedback TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    CHECK (status IN ('pending', 'approved', 'rejected'))
);
```

```sql
CREATE TABLE work_queue (
    task_id INTEGER PRIMARY KEY,
    queued_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);
```

### Runtime behavior

- The database is copied from the host before each agent step and mounted at
  `/state/tasks.db` in the container.
- After the agent exits, the host process captures changes to the database and
  updates the monitoring UI accordingly.
- When a task is moved to `in-review` status via `latasks review`, the host
  process sends a push notification to the monitoring UI.
- Task IDs are formatted as `T{id}` when displayed (e.g., task with id=1 is
  shown as T1).
- The work queue is tracked in the `work_queue` table. Tasks are added to the
  queue via `latasks queue <task_id>`.
- When `latasks next` retrieves the next task, it queries tasks from the queue
  in the following order:
  1. Subtasks are prioritized before their parent tasks (tasks with a
     `parent_id` come before tasks that have children).
  2. Among tasks at the same level, order by task ID ascending.
- Completed tasks are automatically removed from the work queue when their
  status is updated to `completed`.
- Parent tasks should not be marked as `completed` until all child tasks are
  completed (enforced by the host process or within the Go application logic).

## Development Tasks

**High Priority:**
- ✅ T1: Create Go project structure and main.go with CLI setup
- ✅ T2: Implement SQLite database connection and schema initialization
- ✅ T3: Implement 'latasks add' command to create new tasks
- ✅ T6: Implement 'latasks update' command to change task status
- ✅ T8: Implement 'latasks next' command with proper queue ordering
- ✅ T12: Add validation to prevent completing parent tasks before child tasks

**Medium Priority:**
- ✅ T4: Implement 'latasks list' command to display all tasks
- ✅ T5: Implement 'latasks view' command to show task details
- ✅ T7: Implement 'latasks queue' command to add tasks to work queue
- ✅ T9: Implement 'latasks log' command to add task logs
- ✅ T10: Implement 'latasks review' command for review requests
- ✅ T13: Update agents/Dockerfile.opencode to build and install latasks binary
- ✅ T14: Add comprehensive error handling and user-friendly messages

**Low Priority:**
- ✅ T11: Implement 'latasks delete' command to remove tasks
- T15: Write tests for all command functionality
