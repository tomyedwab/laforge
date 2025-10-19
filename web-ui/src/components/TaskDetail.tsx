// Preact JSX doesn't require h import
import { useState, useEffect } from 'preact/hooks';
import { useWebSocket } from '../hooks/useWebSocket';
import type { Task, TaskLog, TaskReview } from '../types';
import { apiService } from '../services/api';
import { TaskForm } from './TaskForm';
import { ReviewRequest } from './ReviewRequest';
import { ReviewDetail } from './ReviewDetail';

interface TaskDetailProps {
  task: Task;
  onClose: () => void;
  onStatusChange?: (taskId: number, status: Task['status']) => void;
  onTaskUpdate?: (updatedTask: Task) => void;
}

export function TaskDetail({ task, onClose, onStatusChange, onTaskUpdate }: TaskDetailProps) {
  const [logs, setLogs] = useState<TaskLog[]>([]);
  const [reviews, setReviews] = useState<TaskReview[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'details' | 'logs' | 'reviews' | 'children'>('details');
  const [isEditing, setIsEditing] = useState(false);
  const [showReviewRequest, setShowReviewRequest] = useState(false);
  const [selectedReview, setSelectedReview] = useState<TaskReview | null>(null);

  // Set up WebSocket connection for real-time updates
  const { isConnected } = useWebSocket({
    onTaskUpdate: (updatedTask) => {
      // If this task is updated, notify the parent component
      if (updatedTask.id === task.id) {
        onTaskUpdate?.(updatedTask);
      }
    },
    onReviewUpdate: (updatedReview) => {
      // If a review for this task is updated, refresh the reviews
      if (updatedReview.task_id === task.id) {
        loadTaskDetails();
      }
    },
  });

  useEffect(() => {
    loadTaskDetails();
  }, [task.id]);

  const loadTaskDetails = async () => {
    try {
      setError(null);

      const [logsResponse, reviewsResponse] = await Promise.all([
        apiService.getTaskLogs(task.id),
        apiService.getTaskReviews(task.id),
      ]);

      setLogs(logsResponse.logs);
      setReviews(reviewsResponse.reviews);
    } catch (error) {
      console.error('Failed to load task details:', error);
      setError('Failed to load task details');
    }
  };

  const handleStatusChange = async (newStatus: Task['status']) => {
    try {
      await apiService.updateTaskStatus(task.id, newStatus);
      onStatusChange?.(task.id, newStatus);
    } catch (error) {
      console.error('Failed to update task status:', error);
      setError('Failed to update task status');
    }
  };

  const handleEdit = () => {
    setIsEditing(true);
  };

  const handleCancelEdit = () => {
    setIsEditing(false);
  };

  const handleTaskUpdate = (updatedTask: Task) => {
    setIsEditing(false);
    onTaskUpdate?.(updatedTask);
    // Reload task details to reflect changes
    loadTaskDetails();
  };

  const handleRequestReview = () => {
    setShowReviewRequest(true);
  };

  const handleReviewCreated = (newReview: TaskReview) => {
    setReviews([...reviews, newReview]);
    setShowReviewRequest(false);
  };

  const handleReviewClick = (review: TaskReview) => {
    setSelectedReview(review);
  };

  const handleReviewUpdated = (updatedReview: TaskReview) => {
    setReviews(reviews.map(r => r.id === updatedReview.id ? updatedReview : r));
    setSelectedReview(null);
  };

  const hasPendingReviews = reviews.some(r => r.status === 'pending');
  const canRequestReview = task.status !== 'completed' && !hasPendingReviews;

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString();
  };

  const getStatusColor = (status: Task['status']) => {
    const colors = {
      'todo': '#95a5a6',
      'in-progress': '#f39c12',
      'in-review': '#9b59b6',
      'completed': '#27ae60',
    };
    return colors[status];
  };

  const getTypeColor = (type: Task['type']) => {
    const colors = {
      'EPIC': '#e74c3c',
      'FEAT': '#3498db',
      'BUG': '#e67e22',
      'PLAN': '#9b59b6',
      'DOC': '#34495e',
      'ARCH': '#16a085',
      'DESIGN': '#d35400',
      'TEST': '#27ae60',
    };
    return colors[type];
  };

  const tabs = [
    { id: 'details', label: 'Details', count: null },
    { id: 'logs', label: 'Logs', count: logs.length },
    { id: 'reviews', label: 'Reviews', count: reviews.length },
    { id: 'children', label: 'Subtasks', count: task.children?.length || 0 },
  ];

  return (
    <div class="task-detail-overlay" onClick={onClose}>
      <div class="task-detail-modal" onClick={(e) => e.stopPropagation()}>
        {isEditing ? (
          <TaskForm
            task={task}
            onSave={handleTaskUpdate}
            onCancel={handleCancelEdit}
          />
        ) : (
          <>
            <div class="task-detail-header">
              <div class="task-detail-title-section">
                <h2>{task.title}</h2>
                <div class="task-detail-badges">
                  <div class={`connection-status small ${isConnected ? 'connected' : 'disconnected'}`}>
                    <span class="status-indicator"></span>
                  </div>
                  <span 
                    class="task-type-badge"
                    style={{ backgroundColor: getTypeColor(task.type), color: 'white' }}
                  >
                    {task.type}
                  </span>
                  <span 
                    class="task-status-badge"
                    style={{ backgroundColor: getStatusColor(task.status), color: 'white' }}
                  >
                    {task.status.replace('-', ' ')}
                  </span>
                  {task.review_required && (
                    <span class="review-required-badge">Review Required</span>
                  )}
                  {hasPendingReviews && (
                    <span class="pending-review-badge">Pending Review</span>
                  )}
                </div>
              </div>
              
              <div class="task-detail-actions">
                <button class="edit-button" onClick={handleEdit}>Edit</button>
                {canRequestReview && (
                  <button class="review-button" onClick={handleRequestReview}>
                    Request Review
                  </button>
                )}
                <button class="close-button" onClick={onClose}>×</button>
              </div>
            </div>

        <div class="task-detail-content">
          <div class="task-detail-tabs">
            {tabs.map(tab => (
              <button
                key={tab.id}
                class={`tab-button ${activeTab === tab.id ? 'active' : ''}`}
                onClick={() => setActiveTab(tab.id as any)}
              >
                {tab.label}
                {tab.count !== null && tab.count > 0 && (
                  <span class="tab-count">{tab.count}</span>
                )}
              </button>
            ))}
          </div>

          <div class="task-detail-panel">
            {activeTab === 'details' && (
              <div class="task-details-tab">
                {task.description && (
                  <div class="detail-section">
                    <h3>Description</h3>
                    <p class="task-description">{task.description}</p>
                  </div>
                )}

                <div class="detail-section">
                  <h3>Properties</h3>
                  <div class="properties-grid">
                    <div class="property-item">
                      <label>ID:</label>
                      <span>{task.id}</span>
                    </div>
                    <div class="property-item">
                      <label>Created:</label>
                      <span>{formatDate(task.created_at)}</span>
                    </div>
                    <div class="property-item">
                      <label>Updated:</label>
                      <span>{formatDate(task.updated_at)}</span>
                    </div>
                    {task.completed_at && (
                      <div class="property-item">
                        <label>Completed:</label>
                        <span>{formatDate(task.completed_at)}</span>
                      </div>
                    )}
                    {task.parent_id && (
                      <div class="property-item">
                        <label>Parent ID:</label>
                        <span>{task.parent_id}</span>
                      </div>
                    )}
                    {task.upstream_dependency_id && (
                      <div class="property-item">
                        <label>Depends on:</label>
                        <span>{task.upstream_dependency_id}</span>
                      </div>
                    )}
                  </div>
                </div>

                <div class="detail-section">
                  <h3>Status Management</h3>
                  <div class="status-controls">
                    <select
                      value={task.status}
                      onChange={(e) => handleStatusChange((e.target as HTMLSelectElement).value as Task['status'])}
                    >
                      <option value="todo">Todo</option>
                      <option value="in-progress">In Progress</option>
                      <option value="in-review">In Review</option>
                      <option value="completed">Completed</option>
                    </select>
                  </div>
                </div>
              </div>
            )}

            {activeTab === 'logs' && (
              <div class="task-logs-tab">
                {logs.length === 0 ? (
                  <p class="empty-message">No logs for this task yet.</p>
                ) : (
                  <div class="logs-list">
                    {logs.map(log => (
                      <div key={log.id} class="log-item">
                        <div class="log-message">{log.message}</div>
                        <div class="log-date">{formatDate(log.created_at)}</div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {activeTab === 'reviews' && (
              <div class="task-reviews-tab">
                {reviews.length === 0 ? (
                  <div class="empty-reviews">
                    <p class="empty-message">No reviews for this task yet.</p>
                    {canRequestReview && (
                      <button class="request-review-button" onClick={handleRequestReview}>
                        Request First Review
                      </button>
                    )}
                  </div>
                ) : (
                  <div class="reviews-container">
                    <div class="reviews-header">
                      <h4>Review History</h4>
                      {canRequestReview && (
                        <button class="request-review-button" onClick={handleRequestReview}>
                          Request New Review
                        </button>
                      )}
                    </div>
                    <div class="reviews-list">
                      {reviews.map(review => (
                        <div 
                          key={review.id} 
                          class="review-item"
                          onClick={() => handleReviewClick(review)}
                        >
                          <div class="review-header">
                            <span class={`review-status status-${review.status}`}>
                              {review.status}
                            </span>
                            <span class="review-date">{formatDate(review.created_at)}</span>
                          </div>
                          <div class="review-message">{review.message}</div>
                          {review.attachment && (
                            <div class="review-attachment">
                              <span class="attachment-icon">📎</span>
                              {review.attachment}
                            </div>
                          )}
                          {review.feedback && (
                            <div class="review-feedback">
                              <strong>Feedback:</strong> {review.feedback}
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )}

            {activeTab === 'children' && (
              <div class="task-children-tab">
                {(!task.children || task.children.length === 0) ? (
                  <p class="empty-message">No subtasks for this task.</p>
                ) : (
                  <div class="children-list">
                    {task.children!.map(child => (
                      <div key={child.id} class="child-task-item">
                        <div class="child-task-header">
                          <span class="child-task-title">{child.title}</span>
                          <span 
                            class="child-task-status"
                            style={{ backgroundColor: getStatusColor(child.status), color: 'white' }}
                          >
                            {child.status.replace('-', ' ')}
                          </span>
                        </div>
                        <div class="child-task-type">{child.type}</div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>

            {error && (
              <div class="error-message">{error}</div>
            )}
          </>
        )}

        {showReviewRequest && (
          <ReviewRequest
            task={task}
            onClose={() => setShowReviewRequest(false)}
            onReviewCreated={handleReviewCreated}
          />
        )}

        {selectedReview && (
          <ReviewDetail
            review={selectedReview}
            onClose={() => setSelectedReview(null)}
            onReviewUpdated={handleReviewUpdated}
          />
        )}
          </>
        )}
      </div>
    </div>
  );
}