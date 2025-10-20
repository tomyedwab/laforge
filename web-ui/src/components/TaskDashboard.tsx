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
  const [upcomingTasks, setUpcomingTasks] = useState<Task[]>([]);
  const [completedTasks, setCompletedTasks] = useState<Task[]>([]);
  const [upcomingPagination, setUpcomingPagination] = useState({ page: 1, limit: 25, total: 0, pages: 1 });
  const [completedPagination, setCompletedPagination] = useState({ page: 1, limit: 25, total: 0, pages: 1 });
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);
  const [activeTab, setActiveTab] = useState<'upcoming' | 'completed'>('upcoming');
  const [filters, setFilters] = useState<TaskFilterOptions>({
    sortBy: 'created_at',
    sortOrder: 'desc',
  });
  const [upcomingPage, setUpcomingPage] = useState(1);
  const [completedPage, setCompletedPage] = useState(1);
  const [itemsPerPage, setItemsPerPage] = useState(25);
  const [isCreatingTask, setIsCreatingTask] = useState(false);

  // Set up WebSocket connection for real-time updates
  const { isConnected, connectionError } = useWebSocket({
    onTaskUpdate: (updatedTask) => {
      // Update the task in the appropriate local state
      if (updatedTask.status === 'completed') {
        // Remove from upcoming if it exists there
        setUpcomingTasks(prevTasks => {
          const filtered = prevTasks.filter(task => task.id !== updatedTask.id);
          if (filtered.length < prevTasks.length) {
            // Task was removed from upcoming, add to completed and update pagination
            setCompletedTasks(prev => [updatedTask, ...prev]);
            setUpcomingPagination(prev => ({ ...prev, total: prev.total - 1 }));
            setCompletedPagination(prev => ({ ...prev, total: prev.total + 1 }));
          }
          return filtered;
        });
        // Update in completed if it exists there
        setCompletedTasks(prevTasks => 
          prevTasks.map(task => 
            task.id === updatedTask.id ? updatedTask : task
          )
        );
      } else {
        // Remove from completed if it exists there
        setCompletedTasks(prevTasks => {
          const filtered = prevTasks.filter(task => task.id !== updatedTask.id);
          if (filtered.length < prevTasks.length) {
            // Task was removed from completed, add to upcoming and update pagination
            setUpcomingTasks(prev => [updatedTask, ...prev]);
            setCompletedPagination(prev => ({ ...prev, total: prev.total - 1 }));
            setUpcomingPagination(prev => ({ ...prev, total: prev.total + 1 }));
          }
          return filtered;
        });
        // Update in upcoming if it exists there
        setUpcomingTasks(prevTasks => 
          prevTasks.map(task => 
            task.id === updatedTask.id ? updatedTask : task
          )
        );
      }
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
      
      // Load tasks based on active tab
      if (activeTab === 'upcoming') {
        // Load upcoming tasks (todo, in-progress, in-review)
        const response = await apiService.getTasks({
          include_children: true,
          include_logs: false,
          include_reviews: false,
          page: upcomingPage,
          limit: itemsPerPage,
          status: 'todo,in-progress,in-review',
          type: filters.type,
        });
        
        setUpcomingTasks(response.tasks);
        setUpcomingPagination(response.pagination);
      } else {
        // Load completed tasks
        const response = await apiService.getTasks({
          include_children: true,
          include_logs: false,
          include_reviews: false,
          page: completedPage,
          limit: itemsPerPage,
          status: 'completed',
          type: filters.type,
        });
        
        setCompletedTasks(response.tasks);
        setCompletedPagination(response.pagination);
      }
    } catch (error) {
      console.error('Failed to load tasks:', error);
      setError('Failed to load tasks. Please try again.');
    } finally {
      setIsLoading(false);
    }
  };

  // Get current tasks and pagination based on active tab
  const currentTasks = activeTab === 'upcoming' ? upcomingTasks : completedTasks;
  const currentPagination = activeTab === 'upcoming' ? upcomingPagination : completedPagination;
  const currentPage = activeTab === 'upcoming' ? upcomingPage : completedPage;

  // Apply local filtering and sorting for more responsive UI
  const processedTasks = useMemo(() => {
    let filtered = currentTasks;

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
  }, [currentTasks, filters]);

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
    // Update the task in the appropriate local state
    if (updatedTask.status === 'completed') {
      setCompletedTasks(prevTasks => 
        prevTasks.map(task => 
          task.id === updatedTask.id ? updatedTask : task
        )
      );
    } else {
      setUpcomingTasks(prevTasks => 
        prevTasks.map(task => 
          task.id === updatedTask.id ? updatedTask : task
        )
      );
    }
  };

  const handleStatusChange = async (taskId: number, status: TaskStatus) => {
    try {
      await apiService.updateTaskStatus(taskId, status);
      
      // If status changed to completed, move task from upcoming to completed
      if (status === 'completed') {
        const task = upcomingTasks.find(t => t.id === taskId);
        if (task) {
          setUpcomingTasks(prev => prev.filter(t => t.id !== taskId));
          setCompletedTasks(prev => [{ ...task, status }, ...prev]);
          // Update pagination totals
          setUpcomingPagination(prev => ({ ...prev, total: prev.total - 1 }));
          setCompletedPagination(prev => ({ ...prev, total: prev.total + 1 }));
        }
      } else {
        // If status changed from completed, move task from completed to upcoming
        const task = completedTasks.find(t => t.id === taskId);
        if (task) {
          setCompletedTasks(prev => prev.filter(t => t.id !== taskId));
          setUpcomingTasks(prev => [{ ...task, status }, ...prev]);
          // Update pagination totals
          setCompletedPagination(prev => ({ ...prev, total: prev.total - 1 }));
          setUpcomingPagination(prev => ({ ...prev, total: prev.total + 1 }));
        }
      }
    } catch (error) {
      console.error('Failed to update task status:', error);
      setError('Failed to update task status');
    }
  };

  const handlePageChange = (page: number) => {
    if (activeTab === 'upcoming') {
      setUpcomingPage(page);
    } else {
      setCompletedPage(page);
    }
    loadTasks();
  };

  const handleItemsPerPageChange = (items: number) => {
    setItemsPerPage(items);
    setUpcomingPage(1);
    setCompletedPage(1);
    loadTasks();
  };

  // Reload tasks when filters change (with debouncing for search)
  useEffect(() => {
    const timeoutId = setTimeout(() => {
      loadTasks();
    }, 300);
    
    return () => clearTimeout(timeoutId);
  }, [filters.status, filters.type, upcomingPage, completedPage, itemsPerPage, activeTab]);

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
            <span class="tab-count">{upcomingPagination.total}</span>
          </button>
          <button class="tab-button" disabled>
            Completed
            <span class="tab-count">{completedPagination.total}</span>
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
          <span class="tab-count">{upcomingPagination.total}</span>
        </button>
        <button
          class={`tab-button ${activeTab === 'completed' ? 'active' : ''}`}
          onClick={() => setActiveTab('completed')}
        >
          Completed
          <span class="tab-count">{completedPagination.total}</span>
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
            totalPages={currentPagination.pages}
            totalItems={currentPagination.total}
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