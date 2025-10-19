// Preact JSX doesn't require h import
import type { Task, TaskStatus, TaskType } from '../types';

interface TaskCardProps {
  task: Task;
  onClick?: (task: Task) => void;
  onStatusChange?: (taskId: number, status: TaskStatus) => void;
  showActions?: boolean;
  isSelected?: boolean;
  depth?: number;
}

const statusColors = {
  'todo': '#95a5a6',
  'in-progress': '#f39c12',
  'in-review': '#9b59b6',
  'completed': '#27ae60',
};

const typeColors = {
  'EPIC': '#e74c3c',
  'FEAT': '#3498db',
  'BUG': '#e67e22',
  'PLAN': '#9b59b6',
  'DOC': '#34495e',
  'ARCH': '#16a085',
  'DESIGN': '#d35400',
  'TEST': '#27ae60',
};

export function TaskCard({ 
  task, 
  onClick, 
  onStatusChange, 
  showActions = true,
  isSelected = false,
  depth = 0,
}: TaskCardProps) {
  const handleStatusChange = (newStatus: TaskStatus) => {
    if (onStatusChange) {
      onStatusChange(task.id, newStatus);
    }
  };

  const getStatusIcon = (status: TaskStatus) => {
    switch (status) {
      case 'todo':
        return '○';
      case 'in-progress':
        return '◐';
      case 'in-review':
        return '◑';
      case 'completed':
        return '●';
      default:
        return '○';
    }
  };

  const getTypeIcon = (type: TaskType) => {
    switch (type) {
      case 'EPIC':
        return '🎯';
      case 'FEAT':
        return '✨';
      case 'BUG':
        return '🐛';
      case 'PLAN':
        return '📋';
      case 'DOC':
        return '📚';
      case 'ARCH':
        return '🏗️';
      case 'DESIGN':
        return '🎨';
      case 'TEST':
        return '🧪';
      default:
        return '📝';
    }
  };

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    const now = new Date();
    const diffTime = Math.abs(now.getTime() - date.getTime());
    const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));
    
    if (diffDays === 0) {
      return 'Today';
    } else if (diffDays === 1) {
      return 'Yesterday';
    } else if (diffDays < 7) {
      return `${diffDays} days ago`;
    } else if (diffDays < 30) {
      const weeks = Math.floor(diffDays / 7);
      return `${weeks} week${weeks > 1 ? 's' : ''} ago`;
    } else {
      return date.toLocaleDateString();
    }
  };

  const hasChildren = task.children && task.children.length > 0;
  const isOverdue = task.status !== 'completed' && task.completed_at && new Date(task.completed_at) < new Date();

  return (
    <div
      class={`task-card ${isSelected ? 'selected' : ''} ${depth > 0 ? 'child-task' : ''}`}
      style={{ marginLeft: `${depth * 24}px` }}
      onClick={() => onClick?.(task)}
    >
      <div class="task-card-header">
        <div class="task-title-section">
          <span class="task-type-icon" title={task.type}>
            {getTypeIcon(task.type)}
          </span>
          <h3 class="task-title">{task.title}</h3>
          {task.review_required && (
            <span class="review-required-badge" title="Review required">
              👁️
            </span>
          )}
          {isOverdue && (
            <span class="overdue-badge" title="Overdue">
              ⚠️
            </span>
          )}
        </div>
        
        <div class="task-status-section">
          {showActions && onStatusChange ? (
            <select
              class="status-select"
              value={task.status}
              onChange={(e) => handleStatusChange((e.target as HTMLSelectElement).value as TaskStatus)}
              onClick={(e) => e.stopPropagation()}
              style={{ backgroundColor: statusColors[task.status] }}
            >
              <option value="todo">Todo</option>
              <option value="in-progress">In Progress</option>
              <option value="in-review">In Review</option>
              <option value="completed">Completed</option>
            </select>
          ) : (
            <span 
              class="task-status-badge"
              style={{ backgroundColor: statusColors[task.status] }}
            >
              {getStatusIcon(task.status)} {task.status.replace('-', ' ')}
            </span>
          )}
        </div>
      </div>

      {task.description && (
        <p class="task-description">{task.description}</p>
      )}

      <div class="task-meta">
        <div class="task-meta-left">
          <span class="task-type-badge" style={{ color: typeColors[task.type] }}>
            {task.type}
          </span>
          {hasChildren && (
            <span class="child-count" title={`${task.children!.length} subtask${task.children!.length !== 1 ? 's' : ''}`}>
              📂 {task.children!.length}
            </span>
          )}
        </div>
        
        <div class="task-meta-right">
          <span class="task-date" title={new Date(task.created_at).toLocaleString()}>
            Created {formatDate(task.created_at)}
          </span>
          {task.updated_at !== task.created_at && (
            <span class="task-date" title={new Date(task.updated_at).toLocaleString()}>
              • Updated {formatDate(task.updated_at)}
            </span>
          )}
        </div>
      </div>

      {showActions && (
        <div class="task-actions" onClick={(e) => e.stopPropagation()}>
          <button
            class="task-action-button view-button"
            onClick={() => onClick?.(task)}
            title="View details"
          >
            👁️ View
          </button>
          <button
            class="task-action-button edit-button"
            title="Edit task"
          >
            ✏️ Edit
          </button>
        </div>
      )}
    </div>
  );
}