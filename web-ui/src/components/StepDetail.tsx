import { h } from 'preact';
import type { Step } from '../types';

interface StepDetailProps {
  step: Step;
  onClose: () => void;
  formatDuration: (durationMs: number) => string;
  formatCost: (cost: number) => string;
}

export function StepDetail({ step, onClose, formatDuration, formatCost }: StepDetailProps) {
  const getStatusInfo = () => {
    if (!step.active) return { text: 'Rolled Back', class: 'status-rolled-back' };
    if (!step.end_time) return { text: 'Running', class: 'status-running' };
    if (step.exit_code === 0) return { text: 'Success', class: 'status-success' };
    return { text: 'Failed', class: 'status-failed' };
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString();
  };

  const getCommitShortSha = (sha: string) => {
    return sha.substring(0, 8);
  };

  const statusInfo = getStatusInfo();

  return (
    <div class="modal-overlay" onClick={onClose}>
      <div class="modal-content" onClick={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <h3>Step S{step.id} Details</h3>
          <button class="modal-close" onClick={onClose}>×</button>
        </div>
        
        <div class="modal-body">
          <div class="detail-section">
            <h4>Status Information</h4>
            <div class={`status-badge ${statusInfo.class}`}>
              {statusInfo.text}
            </div>
            {step.exit_code !== null && (
              <div class="exit-code">
                <strong>Exit Code:</strong> {step.exit_code}
              </div>
            )}
          </div>
          
          <div class="detail-section">
            <h4>Timing Information</h4>
            <div class="info-grid">
              <div class="info-item">
                <strong>Start Time:</strong>
                <span>{formatDate(step.start_time)}</span>
              </div>
              {step.end_time && (
                <div class="info-item">
                  <strong>End Time:</strong>
                  <span>{formatDate(step.end_time)}</span>
                </div>
              )}
              {step.duration_ms && (
                <div class="info-item">
                  <strong>Duration:</strong>
                  <span>{formatDuration(step.duration_ms)}</span>
                </div>
              )}
            </div>
          </div>
          
          <div class="detail-section">
            <h4>Commit Information</h4>
            <div class="info-grid">
              <div class="info-item">
                <strong>Commit Before:</strong>
                <code class="commit-sha">{step.commit_before}</code>
              </div>
              {step.commit_after && (
                <div class="info-item">
                  <strong>Commit After:</strong>
                  <code class="commit-sha">{step.commit_after}</code>
                </div>
              )}
            </div>
          </div>
          
          <div class="detail-section">
            <h4>Token Usage & Cost</h4>
            <div class="info-grid">
              <div class="info-item">
                <strong>Prompt Tokens:</strong>
                <span>{step.prompt_tokens.toLocaleString()}</span>
              </div>
              <div class="info-item">
                <strong>Completion Tokens:</strong>
                <span>{step.completion_tokens.toLocaleString()}</span>
              </div>
              <div class="info-item">
                <strong>Total Tokens:</strong>
                <span>{step.total_tokens.toLocaleString()}</span>
              </div>
              <div class="info-item">
                <strong>Estimated Cost:</strong>
                <span class="cost-value">{formatCost(step.cost_usd)}</span>
              </div>
            </div>
          </div>
          
          {step.parent_step_id && (
            <div class="detail-section">
              <h4>Relationships</h4>
              <div class="info-item">
                <strong>Parent Step:</strong>
                <span>S{step.parent_step_id}</span>
              </div>
            </div>
          )}
          
          <div class="detail-section">
            <h4>Agent Configuration</h4>
            <div class="agent-config">
              <pre>{JSON.stringify(step.agent_config, null, 2)}</pre>
            </div>
          </div>
          
          <div class="detail-section">
            <h4>Project Information</h4>
            <div class="info-item">
              <strong>Project ID:</strong>
              <span>{step.project_id}</span>
            </div>
          </div>
        </div>
        
        <div class="modal-footer">
          <button class="button-secondary" onClick={onClose}>
            Close
          </button>
        </div>
      </div>
    </div>
  );
}