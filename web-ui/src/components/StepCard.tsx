import { h } from 'preact';
import type { Step } from '../types';

interface StepCardProps {
  step: Step;
  onClick: (step: Step) => void;
  formatDuration: (durationMs: number) => string;
  formatCost: (cost: number) => string;
}

export function StepCard({ step, onClick, formatDuration, formatCost }: StepCardProps) {
  const getStatusIcon = () => {
    if (!step.active) return '⚠️';
    if (!step.end_time) return '⏳';
    if (step.exit_code === 0) return '✅';
    return '❌';
  };

  const getStatusClass = () => {
    if (!step.active) return 'step-status-rolled-back';
    if (!step.end_time) return 'step-status-running';
    if (step.exit_code === 0) return 'step-status-success';
    return 'step-status-failed';
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString();
  };

  const getCommitShortSha = (sha: string) => {
    return sha.substring(0, 8);
  };

  return (
    <div 
      class={`step-card ${getStatusClass()}`}
      onClick={() => onClick(step)}
    >
      <div class="step-card-header">
        <div class="step-id">
          <span class="step-id-label">S{step.id}</span>
          <span class="status-icon">{getStatusIcon()}</span>
        </div>
        <div class="step-timing">
          {step.duration_ms && (
            <span class="step-duration">{formatDuration(step.duration_ms)}</span>
          )}
        </div>
      </div>
      
      <div class="step-card-body">
        <div class="step-info">
          <div class="step-date">
            <strong>Started:</strong> {formatDate(step.start_time)}
          </div>
          {step.end_time && (
            <div class="step-date">
              <strong>Completed:</strong> {formatDate(step.end_time)}
            </div>
          )}
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
        
        <div class="step-metrics">
          <div class="token-usage">
            <strong>Tokens:</strong> {step.total_tokens.toLocaleString()}
          </div>
          <div class="step-cost">
            <strong>Cost:</strong> {formatCost(step.cost_usd)}
          </div>
          {step.exit_code !== null && (
            <div class="exit-code">
              <strong>Exit:</strong> {step.exit_code}
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
  );
}