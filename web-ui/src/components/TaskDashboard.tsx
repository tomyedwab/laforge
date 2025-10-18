import { h } from 'preact';
import { useState, useEffect } from 'preact/hooks';
import { apiService } from '../services/api';
import { Task } from '../types';

export function TaskDashboard() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadTasks();
  }, []);

  const loadTasks = async () => {
    try {
      setIsLoading(true);
      setError(null);
      
      const response = await apiService.getTasks({
        include_children: true,
        include_logs: false,
        include_reviews: false,
      });
      
      setTasks(response.tasks);
    } catch (error) {
      console.error('Failed to load tasks:', error);
      setError('Failed to load tasks. Please try again.');
    } finally {
      setIsLoading(false);
    }
  };

  if (isLoading) {
    return (
      <div class="loading-container">
        <div class="loading-spinner">Loading tasks...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div class="error-container">
        <div class="error-message">{error}</div>
        <button onClick={loadTasks}>Retry</button>
      </div>
    );
  }

  return (
    <div class="task-dashboard">
      <h2>Task Dashboard</h2>
      
      {tasks.length === 0 ? (
        <div class="empty-state">
          <p>No tasks found.</p>
          <p>Tasks will appear here once they are created.</p>
        </div>
      ) : (
        <div class="task-list">
          {tasks.map(task => (
            <div key={task.id} class={`task-card task-${task.status}`}>
              <div class="task-header">
                <h3>{task.title}</h3>
                <span class={`task-status status-${task.status}`}>
                  {task.status.replace('-', ' ')}
                </span>
              </div>
              
              {task.description && (
                <p class="task-description">{task.description}</p>
              )}
              
              <div class="task-meta">
                <span class="task-type">{task.type}</span>
                <span class="task-date">
                  Created: {new Date(task.created_at).toLocaleDateString()}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}