import { useState } from 'preact/hooks';
import type { Task } from '../types';
import { apiService } from '../services/api';

interface ReviewRequestProps {
  task: Task;
  onClose: () => void;
  onReviewCreated: (review: any) => void;
}

export function ReviewRequest({ task, onClose, onReviewCreated }: ReviewRequestProps) {
  const [message, setMessage] = useState('');
  const [attachment, setAttachment] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    
    if (!message.trim()) {
      setError('Please provide a review message');
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      const reviewData = {
        message: message.trim(),
        attachment: attachment.trim() || undefined,
      };

      const response = await apiService.createTaskReview(task.id, reviewData);
      onReviewCreated(response.review);
      onClose();
    } catch (error) {
      console.error('Failed to create review request:', error);
      setError('Failed to create review request. Please try again.');
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleAttachmentChange = (e: Event) => {
    const target = e.target as HTMLInputElement;
    setAttachment(target.value);
  };

  const commonArtifactPaths = [
    'docs/artifacts/',
    'docs/specs/',
    'docs/design/',
    'README.md',
    'ARCHITECTURE.md',
  ];

  return (
    <div class="review-request-overlay" onClick={onClose}>
      <div class="review-request-modal" onClick={(e) => e.stopPropagation()}>
        <div class="review-request-header">
          <h3>Request Review for Task</h3>
          <button class="close-button" onClick={onClose}>×</button>
        </div>

        <div class="review-request-content">
          <div class="task-info">
            <h4>{task.title}</h4>
            <p class="task-type">{task.type}</p>
          </div>

          <form onSubmit={handleSubmit}>
            <div class="form-group">
              <label htmlFor="review-message">Review Message *</label>
              <textarea
                id="review-message"
                value={message}
                onChange={(e) => setMessage((e.target as HTMLTextAreaElement).value)}
                placeholder="Please describe what you'd like reviewed..."
                rows={4}
                required
                disabled={isSubmitting}
              />
            </div>

            <div class="form-group">
              <label htmlFor="review-attachment">Artifact Path (Optional)</label>
              <input
                id="review-attachment"
                type="text"
                value={attachment}
                onChange={handleAttachmentChange}
                placeholder="Path to artifact file (e.g., docs/artifacts/design.md)"
                disabled={isSubmitting}
              />
              <div class="attachment-help">
                <p>Common paths:</p>
                <div class="common-paths">
                  {commonArtifactPaths.map(path => (
                    <button
                      key={path}
                      type="button"
                      class="path-suggestion"
                      onClick={() => setAttachment(path)}
                      disabled={isSubmitting}
                    >
                      {path}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            {error && (
              <div class="error-message">{error}</div>
            )}

            <div class="form-actions">
              <button
                type="button"
                class="cancel-button"
                onClick={onClose}
                disabled={isSubmitting}
              >
                Cancel
              </button>
              <button
                type="submit"
                class="submit-button"
                disabled={isSubmitting || !message.trim()}
              >
                {isSubmitting ? 'Creating Review...' : 'Request Review'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}