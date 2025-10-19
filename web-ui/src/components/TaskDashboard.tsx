// Preact JSX doesn't require h import
import { useState, useEffect, useMemo } from 'preact/hooks';
import { apiService } from '../services/api';
import type { Task, TaskStatus } from '../types';
import type { TaskFilterOptions } from './TaskFilters';
import { TaskFilters } from './TaskFilters';
import { TaskCard } from './TaskCard';
import { TaskDetail } from './TaskDetail';
import { TaskForm } from './TaskForm';
import { Pagination } from './Pagination';

interface TaskDashboardProps {
  onTaskClick?: (task: Task) => void;
}

export function TaskDashboard({ onTaskClick }: TaskDashboardProps) {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);
  const [filters, setFilters] = useState<TaskFilterOptions>({
    sortBy: 'created_at',
    sortOrder: 'desc',
  });
  const [currentPage, setCurrentPage] = useState(1);
  const [itemsPerPage, setItemsPerPage] = useState(25);
  const [isCreatingTask, setIsCreatingTask] = useState(false);

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
        page: currentPage,
        limit: itemsPerPage,
        status: filters.status,
        type: filters.type,
      });
      
      setTasks(response.tasks);
    } catch (error) {
      console.error('Failed to load tasks:', error);
      setError('Failed to load tasks. Please try again.');
    } finally {
      setIsLoading(false);
    }
  };

  // Filter and sort tasks locally for more responsive UI
  const processedTasks = useMemo(() => {
    let filtered = tasks;

    // Apply search filter
    if (filters.search) {
      const searchLower = filters.search.toLowerCase();
      filtered = filtered.filter(task => 
        task.title.toLowerCase().includes(searchLower) ||
        task.description?.toLowerCase().includes(searchLower)
      );
    }

    // Apply sorting
    if (filters.sortBy) {
      filtered = [...filtered].sort((a, b) => {
        let aValue: any = a[filters.sortBy!];
        let bValue: any = b[filters.sortBy!];
        
        if (filters.sortBy === 'title') {
          aValue = aValue.toLowerCase();
          bValue = bValue.toLowerCase();
        }
        
        if (aValue < bValue) return filters.sortOrder === 'asc' ? -1 : 1;
        if (aValue > bValue) return filters.sortOrder === 'asc' ? 1 : -1;
        return 0;
      });
    }

    return filtered;
  }, [tasks, filters]);

  const handleTaskClick = (task: Task) => {
    if (onTaskClick) {
      onTaskClick(task);
    } else {
      setSelectedTask(task);
    }
  };

  const handleCreateTask = () => {
    setIsCreatingTask(true);
  };

  const handleCancelCreateTask = () => {
    setIsCreatingTask(false);
  };

  const handleTaskCreated = (newTask: Task) => {
    setIsCreatingTask(false);
    // Reload tasks to include the new task
    loadTasks();
  };

  const handleTaskUpdated = (updatedTask: Task) => {
    // Update the task in the local state
    setTasks(prevTasks => 
      prevTasks.map(task => 
        task.id === updatedTask.id ? updatedTask : task
      )
    );
  };

  const handleStatusChange = async (taskId: number, status: TaskStatus) => {
    try {
      await apiService.updateTaskStatus(taskId, status);
      // Update the task in the local state
      setTasks(prevTasks => 
        prevTasks.map(task => 
          task.id === taskId ? { ...task, status } : task
        )
      );
    } catch (error) {
      console.error('Failed to update task status:', error);
      setError('Failed to update task status');
    }
  };

  const handlePageChange = (page: number) => {
    setCurrentPage(page);
    loadTasks();
  };

  const handleItemsPerPageChange = (items: number) => {
    setItemsPerPage(items);
    setCurrentPage(1);
    loadTasks();
  };

  // Reload tasks when filters change (with debouncing for search)
  useEffect(() => {
    const timeoutId = setTimeout(() => {
      loadTasks();
    }, 300);
    
    return () => clearTimeout(timeoutId);
  }, [filters.status, filters.type, currentPage, itemsPerPage]);

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
      <div class="dashboard-header">
        <h2>Task Dashboard</h2>
        <div class="dashboard-actions">
          <button class="create-task-button" onClick={handleCreateTask}>+ New Task</button>
        </div>
      </div>
      
      <TaskFilters filters={filters} onFiltersChange={setFilters} />
      
      {processedTasks.length === 0 ? (
        <div class="empty-state">
          <p>No tasks found.</p>
          <p>Try adjusting your filters or create a new task.</p>
        </div>
      ) : (
        <>
          <div class="task-list">
            {processedTasks.map(task => (
              <TaskCard
                key={task.id}
                task={task}
                onClick={handleTaskClick}
                onStatusChange={handleStatusChange}
                showActions={true}
              />
            ))}
          </div>
          
          <Pagination
            currentPage={currentPage}
            totalPages={Math.ceil(processedTasks.length / itemsPerPage)}
            totalItems={processedTasks.length}
            itemsPerPage={itemsPerPage}
            onPageChange={handlePageChange}
            onItemsPerPageChange={handleItemsPerPageChange}
          />
        </>
      )}
      
      {selectedTask && (
        <TaskDetail
          task={selectedTask}
          onClose={() => setSelectedTask(null)}
          onStatusChange={handleStatusChange}
          onTaskUpdate={handleTaskUpdated}
        />
      )}
      
      {isCreatingTask && (
        <TaskForm
          onSave={handleTaskCreated}
          onCancel={handleCancelCreateTask}
        />
      )}
    </div>
  );
}