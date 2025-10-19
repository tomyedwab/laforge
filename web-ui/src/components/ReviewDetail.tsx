import { useState } from 'preact/hooks';
import type { TaskReview, ReviewStatus } from '../types';
import { apiService } from '../services/api';
import { ArtifactViewer } from './ArtifactViewer';

interface ReviewDetailProps {
  review: TaskReview;
  onClose: () => void;
  onReviewUpdated: (updatedReview: TaskReview) => void;
}

export function ReviewDetail({ review, onClose, onReviewUpdated }: ReviewDetailProps) {
  const [feedback, setFeedback] = useState('');
  const [status, setStatus] = useState<ReviewStatus>('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showArtifact, setShowArtifact] = useState(false);

  const handleSubmitFeedback = async (e: Event) => {
    e.preventDefault();
    
    if (!status) {
      setError('Please select a review decision');
      return;
    }

    if (status === 'rejected' && !feedback.trim()) {
      setError('Please provide feedback when rejecting a review');
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      const response = await apiService.submitReviewFeedback(review.id, {
        status,
        feedback: feedback.trim() || undefined,
      });
      onReviewUpdated(response.review);
    } catch (error) {
      console.error('Failed to submit review feedback:', error);
      setError('Failed to submit review feedback. Please try again.');
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleStatusChange = (newStatus: ReviewStatus) => {
    setStatus(newStatus);
    if (newStatus === 'approved') {
      setFeedback('');
    }
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString();
  };

  const getStatusColor = (status: ReviewStatus) => {
    const colors = {
      'pending': '#f39c12',
      'approved': '#27ae60',
      'rejected': '#e74c3c',
    };
    return colors[status];
  };

  const isPending = review.status === 'pending';
  const canRespond = isPending && !isSubmitting;

  return (
    <div class="review-detail-overlay" onClick={onClose}>
      <div class="review-detail-modal" onClick={(e) => e.stopPropagation()}>
        <div class="review-detail-header">
          <div class="review-title-section">
            <h3>Review Request</h3>
            <span 
              class="review-status-badge"
              style={{ backgroundColor: getStatusColor(review.status), color: 'white' }}
            >
              {review.status}
            </span>
          </div>
          <button class="close-button" onClick={onClose}>×</button>
        </div>

        <div class="review-detail-content">
          <div class="review-info">
            <div class="review-message">
              <h4>Review Request</h4>
              <p>{review.message}</p>
            </div>

            <div class="review-meta">
              <div class="meta-item">
                <label>Created:</label>
                <span>{formatDate(review.created_at)}</span>
              </div>
              {review.updated_at !== review.created_at && (
                <div class="meta-item">
                  <label>Updated:</label>
                  <span>{formatDate(review.updated_at)}</span>
                </div>
              )}
            </div>

            {review.attachment && (
              <div class="review-attachment">
                <label>Artifact:</label>
                <div class="attachment-info">
                  <span class="attachment-path">{review.attachment}</span>
                  <button
                    class="view-artifact-button"
                    onClick={() => setShowArtifact(true)}
                  >
                    View Artifact
                  </button>
                </div>
              </div>
            )}

            {review.feedback && (
              <div class="review-feedback">
                <h4>Feedback</h4>
                <p>{review.feedback}</p>
              </div>
            )}
          </div>

          {isPending && (
            <div class="review-response">
              <h4>Respond to Review</h4>
              <form onSubmit={handleSubmitFeedback}>
                <div class="form-group">
                  <label>Review Decision *</label>
                  <div class="status-options">
                    <label class="status-option">
                      <input
                        type="radio"
                        name="status"
                        value="approved"
                        checked={status === 'approved'}
                        onChange={() => handleStatusChange('approved')}
                        disabled={!canRespond}
                      />
                      <span class="option-label approve">
                        <span class="option-icon">✓</span>
                        Approve
                      </span>
                    </label>
                    <label class="status-option">
                      <input
                        type="radio"
                        name="status"
                        value="rejected"
                        checked={status === 'rejected'}
                        onChange={() => handleStatusChange('rejected')}
                        disabled={!canRespond}
                      />
                      <span class="option-label reject">
                        <span class="option-icon">✗</span>
                        Request Changes
                      </span>
                    </label>
                  </div>
                </div>

                {status === 'rejected' && (
                  <div class="form-group">
                    <label htmlFor="review-feedback">Feedback *</label>
                    <textarea
                      id="review-feedback"
                      value={feedback}
                      onChange={(e) => setFeedback((e.target as HTMLTextAreaElement).value)}
                      placeholder="Please provide specific feedback on what needs to be changed..."
                      rows={4}
                      required={status === 'rejected'}
                      disabled={!canRespond}
                    />
                  </div>
                )}

                {error && (
                  <div class="error-message">{error}</div>
                )}

                <div class="form-actions">
                  <button
                    type="submit"
                    class="submit-button"
                    disabled={!canRespond || !status}
                  >
                    {isSubmitting ? 'Submitting...' : 'Submit Response'}
                  </button>
                </div>
              </form>
            </div>
          )}

          {!isPending && (
            <div class="review-completed">
              <div class="completion-notice">
                <span class={`completion-icon ${review.status}`}>
                  {review.status === 'approved' ? '✓' : '✗'}
                </span>
                <span class="completion-text">
                  This review has been {review.status}
                </span>
              </div>
            </div>
          )}
        </div>

        {showArtifact && review.attachment && (
          <ArtifactViewer
            artifactPath={review.attachment}
            onClose={() => setShowArtifact(false)}
          />
        )}
      </div>
    </div>
  );
}