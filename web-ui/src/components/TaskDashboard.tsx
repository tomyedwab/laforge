// Preact JSX doesn't require h import
import { useState, useEffect, useMemo } from 'preact/hooks';
import { apiService } from '../services/api';
import { useWebSocket } from '../hooks/useWebSocket';
import type { Task, TaskStatus } from '../types';
import type { TaskFilterOptions } from './TaskFilters';
import { TaskFilters } from './TaskFilters';
import { TaskCard } from './TaskCard';
import { TaskDetail } from './TaskDetail';
import { TaskForm } from './TaskForm';
import { Pagination } from './Pagination';
import { LoadingSpinner, CardSkeleton } from './LoadingStates';

interface TaskDashboardProps {
  onTaskClick?: (task: Task) => void;
}

export function TaskDashboard({ onTaskClick }: TaskDashboardProps) {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);
  const [activeTab, setActiveTab] = useState<'upcoming' | 'completed'>('upcoming');
  const [filters, setFilters] = useState<TaskFilterOptions>({
    sortBy: 'created_at',
    sortOrder: 'desc',
  });
  const [currentPage, setCurrentPage] = useState(1);
  const [itemsPerPage, setItemsPerPage] = useState(25);
  const [isCreatingTask, setIsCreatingTask] = useState(false);

  // Set up WebSocket connection for real-time updates
  const { isConnected, connectionError } = useWebSocket({
    onTaskUpdate: (updatedTask) => {
      // Update the task in the local state
      setTasks(prevTasks => 
        prevTasks.map(task => 
          task.id === updatedTask.id ? updatedTask : task
        )
      );
    },
    onReviewUpdate: (updatedReview) => {
      // If a review is updated, we might need to refresh the task data
      // This is a simple approach - in a real app you might want to be more targeted
      loadTasks();
    },
  });

  useEffect(() => {
    loadTasks();
  }, []);

  const loadTasks = async () => {
    try {
      setIsLoading(true);
      setError(null);
      
      // Determine status filter based on active tab
      let statusFilter: string | undefined;
      if (filters.status) {
        // If user has selected a specific status in filters, use that
        statusFilter = filters.status;
      } else if (activeTab === 'completed') {
        // For completed tab, only show completed tasks
        statusFilter = 'completed';
      }
      // For upcoming tab, we don't set a status filter to show all non-completed tasks
      // The API will return all tasks and we can filter them client-side if needed
      
      const response = await apiService.getTasks({
        include_children: true,
        include_logs: false,
        include_reviews: false,
        page: currentPage,
        limit: itemsPerPage,
        status: statusFilter,
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

    // Apply tab-based filtering
    if (activeTab === 'upcoming') {
      filtered = filtered.filter(task => task.status !== 'completed');
    } else if (activeTab === 'completed') {
      filtered = filtered.filter(task => task.status === 'completed');
    }

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
  }, [tasks, filters, activeTab]);

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
  }, [filters.status, filters.type, currentPage, itemsPerPage, activeTab]);

  if (isLoading) {
    return (
      <div class="task-dashboard">
        <div class="dashboard-header">
          <h2>Task Dashboard</h2>
          <div class="dashboard-actions">
            <div class={`connection-status ${isConnected ? 'connected' : 'disconnected'}`}>
              <span class="status-indicator"></span>
              {isConnected ? 'Connected' : 'Disconnected'}
              {connectionError && <span class="error-tooltip">{connectionError}</span>}
            </div>
          </div>
        </div>
        
        {/* Tab Navigation (disabled during loading) */}
        <div class="task-tabs">
          <button class="tab-button active" disabled>
            Upcoming
            <span class="tab-count">-</span>
          </button>
          <button class="tab-button" disabled>
            Completed
            <span class="tab-count">-</span>
          </button>
        </div>
        
        <CardSkeleton count={5} />
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
          <div class={`connection-status ${isConnected ? 'connected' : 'disconnected'}`}>
            <span class="status-indicator"></span>
            {isConnected ? 'Connected' : 'Disconnected'}
            {connectionError && <span class="error-tooltip">{connectionError}</span>}
          </div>
          <button class="create-task-button" onClick={handleCreateTask}>+ New Task</button>
        </div>
      </div>
      
      {/* Tab Navigation */}
      <div class="task-tabs">
        <button
          class={`tab-button ${activeTab === 'upcoming' ? 'active' : ''}`}
          onClick={() => setActiveTab('upcoming')}
        >
          Upcoming
          <span class="tab-count">{tasks.filter(t => t.status !== 'completed').length}</span>
        </button>
        <button
          class={`tab-button ${activeTab === 'completed' ? 'active' : ''}`}
          onClick={() => setActiveTab('completed')}
        >
          Completed
          <span class="tab-count">{tasks.filter(t => t.status === 'completed').length}</span>
        </button>
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