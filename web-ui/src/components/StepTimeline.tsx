import { h } from 'preact';
import type { Step } from '../types';

interface StepTimelineProps {
  steps: Step[];
  onStepClick: (step: Step) => void;
  formatDuration: (durationMs: number) => string;
  formatCost: (cost: number) => string;
}

export function StepTimeline({ steps, onStepClick, formatDuration, formatCost }: StepTimelineProps) {
  // Group steps by date for better visualization
  const stepsByDate = steps.reduce((acc, step) => {
    const date = new Date(step.start_time).toDateString();
    if (!acc[date]) {
      acc[date] = [];
    }
    acc[date].push(step);
    return acc;
  }, {} as Record<string, Step[]>);

  const getStatusColor = (step: Step) => {
    if (!step.active) return '#ff9800'; // Orange for rolled back
    if (!step.end_time) return '#2196f3'; // Blue for running
    if (step.exit_code === 0) return '#4caf50'; // Green for success
    return '#f44336'; // Red for failed
  };

  const getStatusIcon = (step: Step) => {
    if (!step.active) return '⚠️';
    if (!step.end_time) return '⏳';
    if (step.exit_code === 0) return '✅';
    return '❌';
  };

  const formatTime = (dateString: string) => {
    return new Date(dateString).toLocaleTimeString();
  };

  const getCommitShortSha = (sha: string) => {
    return sha.substring(0, 8);
  };

  // Calculate timeline metrics
  const totalDuration = steps.reduce((sum, step) => sum + (step.duration_ms || 0), 0);
  const totalCost = steps.reduce((sum, step) => sum + step.cost_usd, 0);
  const successRate = steps.length > 0 ? (steps.filter(s => s.exit_code === 0).length / steps.length) * 100 : 0;

  return (
    <div class="step-timeline">
      <div class="timeline-summary">
        <div class="summary-card">
          <h4>Summary</h4>
          <div class="summary-stats">
            <div class="stat">
              <span class="stat-label">Total Steps:</span>
              <span class="stat-value">{steps.length}</span>
            </div>
            <div class="stat">
              <span class="stat-label">Total Duration:</span>
              <span class="stat-value">{formatDuration(totalDuration)}</span>
            </div>
            <div class="stat">
              <span class="stat-label">Total Cost:</span>
              <span class="stat-value">{formatCost(totalCost)}</span>
            </div>
            <div class="stat">
              <span class="stat-label">Success Rate:</span>
              <span class="stat-value">{successRate.toFixed(1)}%</span>
            </div>
          </div>
        </div>
      </div>

      <div class="timeline-container">
        {Object.entries(stepsByDate).map(([date, daySteps]) => (
          <div key={date} class="timeline-day">
            <div class="timeline-date">
              <h4>{new Date(date).toLocaleDateString()}</h4>
              <span class="day-summary">
                {daySteps.length} steps, {formatCost(daySteps.reduce((sum, s) => sum + s.cost_usd, 0))}
              </span>
            </div>
            
            <div class="timeline-steps">
              {daySteps.map((step, index) => (
                <div 
                  key={step.id}
                  class="timeline-step"
                  onClick={() => onStepClick(step)}
                  style={{ borderLeftColor: getStatusColor(step) }}
                >
                  <div class="timeline-step-marker">
                    <span class="status-icon">{getStatusIcon(step)}</span>
                  </div>
                  
                  <div class="timeline-step-content">
                    <div class="step-header">
                      <span class="step-id">S{step.id}</span>
                      <span class="step-time">{formatTime(step.start_time)}</span>
                    </div>
                    
                    <div class="step-details">
                      <div class="step-metric">
                        <strong>Duration:</strong> {step.duration_ms ? formatDuration(step.duration_ms) : 'Running'}
                      </div>
                      <div class="step-metric">
                        <strong>Tokens:</strong> {step.total_tokens.toLocaleString()}
                      </div>
                      <div class="step-metric">
                        <strong>Cost:</strong> {formatCost(step.cost_usd)}
                      </div>
                    </div>
                    
                    <div class="step-commits">
                      <div class="commit-info">
                        <strong>Before:</strong> {getCommitShortSha(step.commit_before)}
                      </div>
                      {step.commit_after && (
                        <div class="commit-info">
                          <strong>After:</strong> {getCommitShortSha(step.commit_after)}
                        </div>
                      )}
                    </div>
                    
                    {step.parent_step_id && (
                      <div class="parent-step">
                        <strong>Parent:</strong> S{step.parent_step_id}
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>

      {steps.length === 0 && (
        <div class="empty-timeline">
          <p>No steps to display</p>
        </div>
      )}
    </div>
  );
}