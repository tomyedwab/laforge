import { h } from 'preact';

interface LoadingSpinnerProps {
  size?: 'small' | 'medium' | 'large';
  message?: string;
  className?: string;
}

export function LoadingSpinner({ size = 'medium', message, className = '' }: LoadingSpinnerProps) {
  const sizeClasses = {
    small: 'loading-spinner-small',
    medium: 'loading-spinner-medium',
    large: 'loading-spinner-large',
  };

  return (
    <div class={`loading-spinner-container ${className}`} role="status" aria-live="polite">
      <div class={`loading-spinner ${sizeClasses[size]}`}>
        <div class="spinner-circle"></div>
        <div class="spinner-circle"></div>
        <div class="spinner-circle"></div>
        <div class="spinner-circle"></div>
      </div>
      {message && (
        <span class="loading-message" aria-label="Loading message">
          {message}
        </span>
      )}
      <span class="sr-only">Loading...</span>
    </div>
  );
}

interface SkeletonLoaderProps {
  lines?: number;
  className?: string;
}

export function SkeletonLoader({ lines = 3, className = '' }: SkeletonLoaderProps) {
  return (
    <div class={`skeleton-loader ${className}`} role="status" aria-label="Loading content">
      {Array.from({ length: lines }, (_, i) => (
        <div key={i} class="skeleton-line"></div>
      ))}
      <span class="sr-only">Loading content...</span>
    </div>
  );
}

interface CardSkeletonProps {
  count?: number;
}

export function CardSkeleton({ count = 3 }: CardSkeletonProps) {
  return (
    <div class="card-skeleton-container" role="status" aria-label="Loading tasks">
      {Array.from({ length: count }, (_, i) => (
        <div key={i} class="card-skeleton">
          <div class="skeleton-header">
            <div class="skeleton-avatar"></div>
            <div class="skeleton-title"></div>
          </div>
          <div class="skeleton-content">
            <div class="skeleton-line long"></div>
            <div class="skeleton-line medium"></div>
            <div class="skeleton-line short"></div>
          </div>
          <div class="skeleton-footer">
            <div class="skeleton-badge"></div>
            <div class="skeleton-date"></div>
          </div>
        </div>
      ))}
      <span class="sr-only">Loading tasks...</span>
    </div>
  );
}

interface ProgressBarProps {
  progress: number;
  message?: string;
  showPercentage?: boolean;
}

export function ProgressBar({ progress, message, showPercentage = true }: ProgressBarProps) {
  const clampedProgress = Math.max(0, Math.min(100, progress));
  
  return (
    <div class="progress-container" role="progressbar" aria-valuenow={clampedProgress} aria-valuemin={0} aria-valuemax={100}>
      {message && <div class="progress-message">{message}</div>}
      <div class="progress-bar">
        <div 
          class="progress-fill" 
          style={{ width: `${clampedProgress}%` }}
          aria-label={`${clampedProgress}% complete`}
        ></div>
      </div>
      {showPercentage && (
        <div class="progress-percentage" aria-label="Progress percentage">
          {clampedProgress}%
        </div>
      )}
    </div>
  );
}

// Screen reader only content
export function ScreenReaderContent({ children }: { children: string }) {
  return <span class="sr-only">{children}</span>;
}