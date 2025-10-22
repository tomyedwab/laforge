import { useState, useEffect } from 'preact/hooks';
import type { Task, TaskReview, ReviewStatus } from '../types';
import { apiService } from '../services/api';
import { ArtifactViewer } from './ArtifactViewer';
import { TaskCard } from './TaskCard';

interface ReviewsDashboardProps {
  onTaskClick?: (task: Task) => void;
}

export function ReviewsDashboard({ onTaskClick }: ReviewsDashboardProps) {
  const [reviews, setReviews] = useState<TaskReview[]>([]);
  const [tasks, setTasks] = useState<Record<number, Task>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState<
    'all' | 'pending' | 'approved' | 'rejected'
  >('all');
  const [sortBy, setSortBy] = useState<'created' | 'updated' | 'status'>(
    'created'
  );
  const [expandedReviews, setExpandedReviews] = useState<Set<number>>(
    new Set()
  );
  const [feedbackText, setFeedbackText] = useState<Record<number, string>>({});
  const [isSubmitting, setIsSubmitting] = useState<Record<number, boolean>>({});
  const [showArtifact, setShowArtifact] = useState<string | null>(null);

  useEffect(() => {
    loadReviews();
  }, []);

  const loadReviews = async () => {
    try {
      setLoading(true);
      setError(null);

      // Get all reviews for the project
      const reviewsResponse = await apiService.getAllProjectReviews({
        limit: 100,
      });

      setReviews(reviewsResponse.reviews);

      // Fetch tasks for the reviews to display task titles
      if (reviewsResponse.reviews.length > 0) {
        const taskIds = [
          ...new Set(reviewsResponse.reviews.map(r => r.task_id)),
        ];
        const tasksMap: Record<number, Task> = {};

        // Fetch each task to get the full details
        for (const taskId of taskIds) {
          try {
            const taskResponse = await apiService.getTask(taskId);
            tasksMap[taskResponse.task.id] = taskResponse.task;
          } catch (err) {
            console.debug(`Failed to load task ${taskId}:`, err);
          }
        }

        setTasks(tasksMap);
      }
    } catch (error) {
      console.error('Failed to load reviews:', error);
      setError('Failed to load reviews');
    } finally {
      setLoading(false);
    }
  };

  const toggleReviewExpanded = (reviewId: number) => {
    const newExpanded = new Set(expandedReviews);
    if (newExpanded.has(reviewId)) {
      newExpanded.delete(reviewId);
    } else {
      newExpanded.add(reviewId);
    }
    setExpandedReviews(newExpanded);
  };

  const handleFeedbackChange = (reviewId: number, feedback: string) => {
    setFeedbackText(prev => ({ ...prev, [reviewId]: feedback }));
  };

  const handleSubmitFeedback = async (reviewId: number, accepted: boolean) => {
    const feedback = feedbackText[reviewId] || '';
    const status = accepted ? 'approved' : 'rejected';

    if (status === 'rejected' && !feedback.trim()) {
      setError('Please provide feedback when rejecting a review');
      return;
    }

    setIsSubmitting(prev => ({ ...prev, [reviewId]: true }));
    setError(null);

    try {
      const response = await apiService.submitReviewFeedback(reviewId, {
        status,
        feedback: feedback.trim() || undefined,
      });

      // Update the review in the list
      setReviews(reviews.map(r => (r.id === reviewId ? response.review : r)));

      // Clear form state for this review
      setFeedbackText(prev => {
        const newState = { ...prev };
        delete newState[reviewId];
        return newState;
      });
    } catch (error) {
      console.error('Failed to submit review feedback:', error);
      setError('Failed to submit review feedback. Please try again.');
    } finally {
      setIsSubmitting(prev => ({ ...prev, [reviewId]: false }));
    }
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
      pending: 'var(--color-warning)',
      approved: 'var(--color-success)',
      rejected: 'var(--color-error)',
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
          return (
            new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
          );
        case 'updated':
          return (
            new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
          );
        case 'status':
          return a.status.localeCompare(b.status);
        default:
          return 0;
      }
    });
  };

  const filteredReviews = sortReviews(
    filter === 'all' ? reviews : reviews.filter(r => r.status === filter)
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

      {error && <div class="error-message">{error}</div>}

      <div class="reviews-controls">
        <div class="filter-controls">
          <label>Filter:</label>
          <select
            value={filter}
            onChange={e =>
              setFilter((e.target as HTMLSelectElement).value as any)
            }
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
            onChange={e =>
              setSortBy((e.target as HTMLSelectElement).value as any)
            }
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
              : `No ${filter} reviews found.`}
          </p>
        </div>
      ) : (
        <div class="reviews-list">
          {filteredReviews.map(review => {
            const task = tasks[review.task_id];
            const isPending = review.status === 'pending';
            const isExpanded = isPending || expandedReviews.has(review.id);
            const canRespond = isPending && !isSubmitting[review.id];
            const currentFeedback = feedbackText[review.id] || '';

            return (
              <div
                key={review.id}
                class={`review-card ${isExpanded ? 'expanded' : ''}`}
              >
                <div class="review-card-header">
                  <div class="review-status">
                    <span class={`status-badge status-${review.status}`}>
                      {review.status}
                    </span>
                    {isPending && (
                      <span class="pending-indicator">⚡ Action Required</span>
                    )}
                  </div>
                  {!isExpanded && (
                    <div class="review-message-short">{review.message}</div>
                  )}
                  <div class="review-header-actions">
                    <div class="review-date">
                      {formatDate(review.created_at)}
                    </div>
                    <button
                      class="expand-button"
                      onClick={() => toggleReviewExpanded(review.id)}
                      aria-label={
                        isExpanded ? 'Collapse review' : 'Expand review'
                      }
                    >
                      {isExpanded ? '▼' : '▶'}
                    </button>
                  </div>
                </div>

                {isExpanded && (
                  <div class="review-card-content">
                    <div class="review-message">{review.message}</div>

                    {task && (
                      <div class="related-task">
                        <span class="task-label">Task:</span>
                        <button
                          class="task-link"
                          onClick={e => {
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
                        <button
                          class="view-artifact-button"
                          onClick={() => setShowArtifact(review.attachment!)}
                        >
                          View Artifact
                        </button>
                      </div>
                    )}

                    {isPending ? (
                      <div class="review-response-inline">
                        <h4>Submit review feedback</h4>
                        <div class="form-group">
                          <label htmlFor={`feedback-${review.id}`}>
                            Feedback *
                          </label>
                          <textarea
                            id={`feedback-${review.id}`}
                            value={currentFeedback}
                            onChange={e =>
                              handleFeedbackChange(
                                review.id,
                                (e.target as HTMLTextAreaElement).value
                              )
                            }
                            placeholder="Please provide specific feedback on what needs to be changed..."
                            rows={4}
                            required={false}
                            disabled={!canRespond}
                          />
                        </div>

                        <div class="form-actions">
                          <button
                            type="button"
                            class="submit-button"
                            onClick={() =>
                              handleSubmitFeedback(review.id, true)
                            }
                            disabled={!canRespond}
                          >
                            {isSubmitting[review.id]
                              ? 'Submitting...'
                              : '✓ Approve'}
                          </button>
                          <button
                            type="button"
                            class="submit-button"
                            onClick={() =>
                              handleSubmitFeedback(review.id, false)
                            }
                            disabled={!canRespond}
                          >
                            {isSubmitting[review.id]
                              ? 'Submitting...'
                              : '✗ Request Changes'}
                          </button>
                        </div>
                      </div>
                    ) : (
                      <div class="review-completed-inline">
                        <div class="completion-notice">
                          <span class={`completion-icon ${review.status}`}>
                            {review.status === 'approved' ? '✓' : '✗'}
                          </span>
                          <span class="completion-text">
                            This review has been {review.status}
                          </span>
                        </div>
                        {review.feedback && (
                          <div class="review-feedback-completed">
                            <strong>Feedback:</strong> {review.feedback}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {showArtifact && (
        <ArtifactViewer
          artifactPath={showArtifact}
          onClose={() => setShowArtifact(null)}
        />
      )}
    </div>
  );
}
