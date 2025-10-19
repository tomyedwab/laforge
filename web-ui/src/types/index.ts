// Task types
export interface Task {
  id: number;
  title: string;
  description: string;
  acceptance_criteria: string;
  type: TaskType;
  status: TaskStatus;
  parent_id: number | null;
  upstream_dependency_id: number | null;
  review_required: boolean;
  created_at: string;
  updated_at: string;
  completed_at: string | null;
  children?: Task[];
  logs?: TaskLog[];
  reviews?: TaskReview[];
}

export type TaskType = 'EPIC' | 'FEAT' | 'BUG' | 'PLAN' | 'DOC' | 'ARCH' | 'DESIGN' | 'TEST';

export type TaskStatus = 'todo' | 'in-progress' | 'in-review' | 'completed';

export interface TaskLog {
  id: number;
  task_id: number;
  message: string;
  created_at: string;
}

export interface TaskReview {
  id: number;
  task_id: number;
  message: string;
  attachment?: string;
  status: ReviewStatus;
  feedback: string | null;
  created_at: string;
  updated_at: string;
}

export type ReviewStatus = 'pending' | 'approved' | 'rejected';

// Step types
export interface Step {
  id: number;
  project_id: string;
  active: boolean;
  parent_step_id: number | null;
  commit_before: string;
  commit_after: string;
  agent_config: Record<string, unknown>;
  start_time: string;
  end_time: string;
  duration_ms: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  cost_usd: number;
  exit_code: number;
}

// API Response types
export interface ApiResponse<T> {
  data: T;
  meta: {
    timestamp: string;
    version: string;
  };
}

export interface ApiError {
  error: {
    code: string;
    message: string;
    details?: Record<string, string>;
  };
  meta: {
    timestamp: string;
    version: string;
  };
}

export interface PaginationInfo {
  page: number;
  limit: number;
  total: number;
  pages: number;
}

export interface PaginatedResponse<T> {
  data: T[];
  pagination: PaginationInfo;
}

// WebSocket message types
export interface WebSocketMessage {
  type: 'task_updated' | 'review_updated' | 'step_completed';
  data: unknown;
}

export interface WebSocketSubscribeMessage {
  type: 'subscribe';
  channels: ('tasks' | 'reviews' | 'steps')[];
}