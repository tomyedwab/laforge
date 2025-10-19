// Preact JSX doesn't require h import
import { useState, useEffect } from 'preact/hooks';
import type { Task, TaskType, TaskStatus } from '../types';
import { apiService } from '../services/api';

interface TaskFormProps {
  task?: Task;
  onSave: (task: Task) => void;
  onCancel: () => void;
}

interface FormData {
  title: string;
  description: string;
  acceptance_criteria: string;
  type: TaskType;
  status: TaskStatus;
  parent_id: number | null;
  upstream_dependency_id: number | null;
  review_required: boolean;
}

interface FormErrors {
  title?: string;
  description?: string;
  acceptance_criteria?: string;
  type?: string;
  status?: string;
}

export function TaskForm({ task, onSave, onCancel }: TaskFormProps) {
  // Extract title without type prefix for existing tasks
  const extractTitleWithoutType = (title: string, type: string): string => {
    const typePrefix = `[${type}] `;
    return title.startsWith(typePrefix) ? title.slice(typePrefix.length) : title;
  };

  const [formData, setFormData] = useState<FormData>({
    title: task ? extractTitleWithoutType(task.title, task.type) : '',
    description: task?.description || '',
    acceptance_criteria: task?.acceptance_criteria || '',
    type: task?.type || 'FEAT',
    status: task?.status || 'todo',
    parent_id: task?.parent_id || null,
    upstream_dependency_id: task?.upstream_dependency_id || null,
    review_required: task?.review_required || false,
  });

  const [errors, setErrors] = useState<FormErrors>({});
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [availableParents, setAvailableParents] = useState<Task[]>([]);
  const [availableDependencies, setAvailableDependencies] = useState<Task[]>([]);

  const taskTypes: TaskType[] = ['EPIC', 'FEAT', 'BUG', 'PLAN', 'DOC', 'ARCH', 'DESIGN', 'TEST'];
  const taskStatuses: TaskStatus[] = ['todo', 'in-progress', 'in-review', 'completed'];

  useEffect(() => {
    loadAvailableTasks();
  }, []);

  const loadAvailableTasks = async () => {
    try {
      const response = await apiService.getTasks({
        status: 'todo,in-progress,in-review',
        limit: 100,
      });
      
      // Filter out the current task if editing
      const availableTasks = task 
        ? response.tasks.filter(t => t.id !== task.id)
        : response.tasks;
      
      setAvailableParents(availableTasks);
      setAvailableDependencies(availableTasks);
    } catch (error) {
      console.error('Failed to load available tasks:', error);
    }
  };

  const validateForm = (): boolean => {
    const newErrors: FormErrors = {};

    if (!formData.title.trim()) {
      newErrors.title = 'Title is required';
    } else if (formData.title.length < 3) {
      newErrors.title = 'Title must be at least 3 characters';
    } else if (formData.title.length > 200) {
      newErrors.title = 'Title must be less than 200 characters';
    }

    if (formData.description && formData.description.length > 2000) {
      newErrors.description = 'Description must be less than 2000 characters';
    }

    if (formData.acceptance_criteria && formData.acceptance_criteria.length > 2000) {
      newErrors.acceptance_criteria = 'Acceptance criteria must be less than 2000 characters';
    }

    if (!formData.type) {
      newErrors.type = 'Type is required';
    }

    if (!formData.status) {
      newErrors.status = 'Status is required';
    }

    setErrors(newErrors);
    return Object.keys(newErrors).length === 0;
  };

  const handleSubmit = async (e: Event) => {
    e.preventDefault();

    if (!validateForm()) {
      return;
    }

    setIsSubmitting(true);

    try {
      let savedTask: Task;

      // Prepare the data to send to the backend
      const taskData = {
        ...formData,
        // Prepend task type to title for new tasks or if title doesn't already have the type prefix
        title: formData.title.startsWith(`[${formData.type}] `) 
          ? formData.title 
          : `[${formData.type}] ${formData.title}`
      };

      if (task) {
        // Update existing task
        const response = await apiService.updateTask(task.id, taskData);
        savedTask = response.task;
      } else {
        // Create new task
        const response = await apiService.createTask(taskData);
        savedTask = response.task;
      }

      onSave(savedTask);
    } catch (error) {
      console.error('Failed to save task:', error);
      setErrors({ 
        title: error instanceof Error ? error.message : 'Failed to save task' 
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleInputChange = (field: keyof FormData, value: any) => {
    setFormData(prev => ({ ...prev, [field]: value }));
    // Clear error for this field when user starts typing
    if (errors[field as keyof FormErrors]) {
      setErrors(prev => ({ ...prev, [field]: undefined }));
    }
  };

  const handleNumberInputChange = (field: 'parent_id' | 'upstream_dependency_id', value: string) => {
    const numValue = value === '' ? null : parseInt(value, 10);
    handleInputChange(field, numValue);
  };

  return (
    <div class="task-form-overlay">
      <div class="task-form-modal">
        <div class="task-form-header">
          <h2>{task ? 'Edit Task' : 'Create New Task'}</h2>
          <button class="close-button" onClick={onCancel}>×</button>
        </div>

        <form class="task-form" onSubmit={handleSubmit}>
          <div class="form-group">
            <label htmlFor="title">Title *</label>
            <input
              id="title"
              type="text"
              value={formData.title}
              onChange={(e) => handleInputChange('title', (e.target as HTMLInputElement).value)}
              class={errors.title ? 'error' : ''}
              placeholder="[FEAT] Implement user authentication"
              disabled={isSubmitting}
              required
            />
            {errors.title && <span class="error-message">{errors.title}</span>}
          </div>

          <div class="form-group">
            <label htmlFor="description">Description</label>
            <textarea
              id="description"
              value={formData.description}
              onChange={(e) => handleInputChange('description', (e.target as HTMLTextAreaElement).value)}
              class={errors.description ? 'error' : ''}
              placeholder="Detailed description of the task..."
              rows={4}
              disabled={isSubmitting}
            />
            {errors.description && <span class="error-message">{errors.description}</span>}
            <div class="character-count">
              {formData.description.length}/2000
            </div>
          </div>

          <div class="form-group">
            <label htmlFor="acceptance_criteria">Acceptance Criteria</label>
            <textarea
              id="acceptance_criteria"
              value={formData.acceptance_criteria}
              onChange={(e) => handleInputChange('acceptance_criteria', (e.target as HTMLTextAreaElement).value)}
              placeholder="Define what needs to be true for this task to be considered complete..."
              rows={4}
              disabled={isSubmitting}
            />
            <div class="character-count">
              {formData.acceptance_criteria.length}/2000
            </div>
          </div>

          <div class="form-row">
            <div class="form-group">
              <label htmlFor="type">Type *</label>
              <select
                id="type"
                value={formData.type}
                onChange={(e) => handleInputChange('type', (e.target as HTMLSelectElement).value as TaskType)}
                class={errors.type ? 'error' : ''}
                disabled={isSubmitting}
                required
              >
                {taskTypes.map(type => (
                  <option key={type} value={type}>{type}</option>
                ))}
              </select>
              {errors.type && <span class="error-message">{errors.type}</span>}
            </div>

            <div class="form-group">
              <label htmlFor="status">Status *</label>
              <select
                id="status"
                value={formData.status}
                onChange={(e) => handleInputChange('status', (e.target as HTMLSelectElement).value as TaskStatus)}
                class={errors.status ? 'error' : ''}
                disabled={isSubmitting}
                required
              >
                {taskStatuses.map(status => (
                  <option key={status} value={status}>
                    {status.replace('-', ' ').replace(/\b\w/g, l => l.toUpperCase())}
                  </option>
                ))}
              </select>
              {errors.status && <span class="error-message">{errors.status}</span>}
            </div>
          </div>

          <div class="form-row">
            <div class="form-group">
              <label htmlFor="parent_id">Parent Task</label>
              <select
                id="parent_id"
                value={formData.parent_id || ''}
                onChange={(e) => handleNumberInputChange('parent_id', (e.target as HTMLSelectElement).value)}
                disabled={isSubmitting}
              >
                <option value="">No parent</option>
                {availableParents.map(parent => (
                  <option key={parent.id} value={parent.id}>
                    {parent.title} ({parent.type})
                  </option>
                ))}
              </select>
            </div>

            <div class="form-group">
              <label htmlFor="upstream_dependency_id">Depends On</label>
              <select
                id="upstream_dependency_id"
                value={formData.upstream_dependency_id || ''}
                onChange={(e) => handleNumberInputChange('upstream_dependency_id', (e.target as HTMLSelectElement).value)}
                disabled={isSubmitting}
              >
                <option value="">No dependency</option>
                {availableDependencies.map(dep => (
                  <option key={dep.id} value={dep.id}>
                    {dep.title} ({dep.type})
                  </option>
                ))}
              </select>
            </div>
          </div>

          <div class="form-group">
            <label class="checkbox-label">
              <input
                type="checkbox"
                checked={formData.review_required}
                onChange={(e) => handleInputChange('review_required', (e.target as HTMLInputElement).checked)}
                disabled={isSubmitting}
              />
              <span>Review Required</span>
            </label>
          </div>

          <div class="form-actions">
            <button
              type="button"
              class="cancel-button"
              onClick={onCancel}
              disabled={isSubmitting}
            >
              Cancel
            </button>
            <button
              type="submit"
              class="save-button"
              disabled={isSubmitting}
            >
              {isSubmitting ? 'Saving...' : (task ? 'Update Task' : 'Create Task')}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}