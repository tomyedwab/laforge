import { useState, useEffect } from 'preact/hooks';
import type { Task, TaskReview } from '../types';
import { apiService } from '../services/api';
import { ReviewDetail } from './ReviewDetail';
import { TaskCard } from './TaskCard';

interface ReviewsDashboardProps {
  onTaskClick?: (task: Task) => void;
}

export function ReviewsDashboard({ onTaskClick }: ReviewsDashboardProps) {
  const [reviews, setReviews] = useState<TaskReview[]>([]);
  const [tasks, setTasks] = useState<Record<number, Task>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedReview, setSelectedReview] = useState<TaskReview | null>(null);
  const [filter, setFilter] = useState<'all' | 'pending' | 'approved' | 'rejected'>('all');
  const [sortBy, setSortBy] = useState<'created' | 'updated' | 'status'>('created');

  useEffect(() => {
    loadReviews();
  }, []);

  const loadReviews = async () => {
    try {
      setLoading(true);
      setError(null);

      // Get all tasks with their reviews
      const tasksResponse = await apiService.getTasks({ 
        include_reviews: true,
        limit: 100 
      });

      const allReviews: TaskReview[] = [];
      const taskMap: Record<number, Task> = {};

      tasksResponse.tasks.forEach(task => {
        taskMap[task.id] = task;
        if (task.reviews) {
          allReviews.push(...task.reviews);
        }
      });

      setReviews(allReviews);
      setTasks(taskMap);
    } catch (error) {
      console.error('Failed to load reviews:', error);
      setError('Failed to load reviews');
    } finally {
      setLoading(false);
    }
  };

  const handleReviewClick = (review: TaskReview) => {
    setSelectedReview(review);
  };

  const handleReviewUpdated = (updatedReview: TaskReview) => {
    setReviews(reviews.map(r => r.id === updatedReview.id ? updatedReview : r));
    setSelectedReview(null);
  };

  const handleTaskClick = (taskId: number) => {
    const task = tasks[taskId];
    if (task && onTaskClick) {
      onTaskClick(task);
    }
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString();
  };

  const getStatusColor = (status: TaskReview['status']) => {
    const colors = {
      'pending': '#f39c12',
      'approved': '#27ae60',
      'rejected': '#e74c3c',
    };
    return colors[status];
  };

  const getStatusStats = () => {
    const stats = {
      pending: reviews.filter(r => r.status === 'pending').length,
      approved: reviews.filter(r => r.status === 'approved').length,
      rejected: reviews.filter(r => r.status === 'rejected').length,
      total: reviews.length,
    };
    return stats;
  };

  const sortReviews = (reviews: TaskReview[]) => {
    return [...reviews].sort((a, b) => {
      switch (sortBy) {
        case 'created':
          return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
        case 'updated':
          return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
        case 'status':
          return a.status.localeCompare(b.status);
        default:
          return 0;
      }
    });
  };

  const filteredReviews = sortReviews(
    filter === 'all' 
      ? reviews 
      : reviews.filter(r => r.status === filter)
  );

  const stats = getStatusStats();

  if (loading) {
    return (
      <div class="reviews-dashboard">
        <div class="loading-container">
          <div class="loading-spinner"></div>
          <p>Loading reviews...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div class="reviews-dashboard">
        <div class="error-container">
          <div class="error-icon">⚠️</div>
          <p>{error}</p>
          <button class="retry-button" onClick={loadReviews}>
            Retry
          </button>
        </div>
      </div>
    );
  }

  return (
    <div class="reviews-dashboard">
      <div class="reviews-header">
        <h2>Review Dashboard</h2>
        <div class="reviews-stats">
          <div class="stat-item">
            <span class="stat-number">{stats.total}</span>
            <span class="stat-label">Total</span>
          </div>
          <div class="stat-item pending">
            <span class="stat-number">{stats.pending}</span>
            <span class="stat-label">Pending</span>
          </div>
          <div class="stat-item approved">
            <span class="stat-number">{stats.approved}</span>
            <span class="stat-label">Approved</span>
          </div>
          <div class="stat-item rejected">
            <span class="stat-number">{stats.rejected}</span>
            <span class="stat-label">Rejected</span>
          </div>
        </div>
      </div>

      <div class="reviews-controls">
        <div class="filter-controls">
          <label>Filter:</label>
          <select 
            value={filter} 
            onChange={(e) => setFilter((e.target as HTMLSelectElement).value as any)}
          >
            <option value="all">All Reviews</option>
            <option value="pending">Pending</option>
            <option value="approved">Approved</option>
            <option value="rejected">Rejected</option>
          </select>
        </div>
        <div class="sort-controls">
          <label>Sort by:</label>
          <select 
            value={sortBy} 
            onChange={(e) => setSortBy((e.target as HTMLSelectElement).value as any)}
          >
            <option value="created">Created Date</option>
            <option value="updated">Updated Date</option>
            <option value="status">Status</option>
          </select>
        </div>
      </div>

      {filteredReviews.length === 0 ? (
        <div class="empty-reviews">
          <div class="empty-icon">📋</div>
          <h3>No reviews found</h3>
          <p>
            {filter === 'all' 
              ? 'There are no reviews yet.' 
              : `No ${filter} reviews found.`
            }
          </p>
        </div>
      ) : (
        <div class="reviews-list">
          {filteredReviews.map(review => {
            const task = tasks[review.task_id];
            return (
              <div 
                key={review.id} 
                class="review-card"
                onClick={() => handleReviewClick(review)}
              >
                <div class="review-card-header">
                  <div class="review-status">
                    <span 
                      class="status-badge"
                      style={{ backgroundColor: getStatusColor(review.status), color: 'white' }}
                    >
                      {review.status}
                    </span>
                    {review.status === 'pending' && (
                      <span class="pending-indicator">⚡ Action Required</span>
                    )}
                  </div>
                  <div class="review-date">
                    {formatDate(review.created_at)}
                  </div>
                </div>

                <div class="review-card-content">
                  <div class="review-message">
                    {review.message}
                  </div>
                  
                  {task && (
                    <div class="related-task">
                      <span class="task-label">Task:</span>
                      <button 
                        class="task-link"
                        onClick={(e) => {
                          e.stopPropagation();
                          handleTaskClick(task.id);
                        }}
                      >
                        {task.title}
                      </button>
                    </div>
                  )}

                  {review.attachment && (
                    <div class="review-attachment">
                      <span class="attachment-icon">📎</span>
                      <span class="attachment-path">{review.attachment}</span>
                    </div>
                  )}

                  {review.feedback && (
                    <div class="review-feedback-preview">
                      <strong>Feedback:</strong> {review.feedback}
                    </div>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {selectedReview && (
        <ReviewDetail
          review={selectedReview}
          onClose={() => setSelectedReview(null)}
          onReviewUpdated={handleReviewUpdated}
        />
      )}
    </div>
  );
}