import type {
  ApiResponse,
  ApiError,
  Task,
  TaskLog,
  TaskReview,
  Step,
} from '../types';

const API_BASE_URL =
  import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api/v1';

class ApiService {
  private projectId: string | null = null;

  constructor() {
    // Load projectId from localStorage on initialization
    const storedProject = localStorage.getItem('laforge_selected_project');
    if (storedProject) {
      try {
        const project = JSON.parse(storedProject);
        if (project && project.id) {
          this.projectId = project.id;
        }
      } catch (err) {
        // Silently ignore parse errors
        console.debug('Failed to parse stored project');
      }
    }
  }

  setProjectId(projectId: string) {
    this.projectId = projectId;
  }

  getProjectId(): string | null {
    return this.projectId;
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const url = `${API_BASE_URL}${endpoint}`;
    const token = localStorage.getItem(
      import.meta.env.VITE_AUTH_TOKEN_KEY || 'laforge_auth_token'
    );

    const headers: HeadersInit = {
      'Content-Type': 'application/json',
      ...options.headers,
    };

    if (token) {
      headers.Authorization = `Bearer ${token}`;
    }

    const response = await fetch(url, {
      ...options,
      headers,
    });

    if (!response.ok) {
      const error: ApiError = await response.json();
      throw new Error(error.error.message);
    }

    const data: ApiResponse<T> = await response.json();
    return data.data;
  }

  // Task endpoints
  async getTasks(params?: {
    status?: string;
    type?: string;
    parent_id?: number;
    include_children?: boolean;
    include_logs?: boolean;
    include_reviews?: boolean;
    page?: number;
    limit?: number;
    search?: string;
  }): Promise<{ tasks: Task[]; pagination: any }> {
    const searchParams = new URLSearchParams();
    Object.entries(params || {}).forEach(([key, value]) => {
      if (value !== undefined) {
        searchParams.append(key, String(value));
      }
    });

    const queryString = searchParams.toString();
    return this.request<{ tasks: Task[]; pagination: any }>(
      `/projects/${this.projectId}/tasks${queryString ? `?${queryString}` : ''}`
    );
  }

  async getTask(
    id: number,
    include?: {
      children?: boolean;
      logs?: boolean;
      reviews?: boolean;
    }
  ): Promise<{ task: Task }> {
    const searchParams = new URLSearchParams();
    Object.entries(include || {}).forEach(([key, value]) => {
      if (value) searchParams.append(`include_${key}`, 'true');
    });

    const queryString = searchParams.toString();
    return this.request<{ task: Task }>(
      `/projects/${this.projectId}/tasks/${id}${queryString ? `?${queryString}` : ''}`
    );
  }

  async createTask(task: Partial<Task>): Promise<{ task: Task }> {
    return this.request<{ task: Task }>(`/projects/${this.projectId}/tasks`, {
      method: 'POST',
      body: JSON.stringify(task),
    });
  }

  async updateTask(id: number, task: Partial<Task>): Promise<{ task: Task }> {
    return this.request<{ task: Task }>(
      `/projects/${this.projectId}/tasks/${id}`,
      {
        method: 'PUT',
        body: JSON.stringify(task),
      }
    );
  }

  async updateTaskStatus(id: number, status: string): Promise<{ task: Task }> {
    return this.request<{ task: Task }>(
      `/projects/${this.projectId}/tasks/${id}/status`,
      {
        method: 'PUT',
        body: JSON.stringify({ status }),
      }
    );
  }

  async deleteTask(id: number): Promise<void> {
    return this.request<void>(`/projects/${this.projectId}/tasks/${id}`, {
      method: 'DELETE',
    });
  }

  async getNextTask(): Promise<{ task: Task | null; message?: string }> {
    return this.request<{ task: Task | null; message?: string }>(
      `/projects/${this.projectId}/tasks/next`
    );
  }

  // Task logs
  async getTaskLogs(
    taskId: number,
    page?: number,
    limit?: number
  ): Promise<{ logs: TaskLog[]; pagination: any }> {
    const searchParams = new URLSearchParams();
    if (page) searchParams.append('page', String(page));
    if (limit) searchParams.append('limit', String(limit));

    const queryString = searchParams.toString();
    return this.request<{ logs: TaskLog[]; pagination: any }>(
      `/projects/${this.projectId}/tasks/${taskId}/logs${queryString ? `?${queryString}` : ''}`
    );
  }

  async addTaskLog(taskId: number, message: string): Promise<{ log: TaskLog }> {
    return this.request<{ log: TaskLog }>(
      `/projects/${this.projectId}/tasks/${taskId}/logs`,
      {
        method: 'POST',
        body: JSON.stringify({ message }),
      }
    );
  }

  // Task reviews
  async getTaskReviews(taskId: number): Promise<{ reviews: TaskReview[] }> {
    return this.request<{ reviews: TaskReview[] }>(
      `/projects/${this.projectId}/tasks/${taskId}/reviews`
    );
  }

  async createTaskReview(
    taskId: number,
    review: {
      message: string;
      attachment?: string;
    }
  ): Promise<{ review: TaskReview }> {
    return this.request<{ review: TaskReview }>(
      `/projects/${this.projectId}/tasks/${taskId}/reviews`,
      {
        method: 'POST',
        body: JSON.stringify(review),
      }
    );
  }

  async submitReviewFeedback(
    reviewId: number,
    feedback: {
      status: 'approved' | 'rejected';
      feedback?: string;
    }
  ): Promise<{ review: TaskReview }> {
    return this.request<{ review: TaskReview }>(
      `/projects/${this.projectId}/reviews/${reviewId}/feedback`,
      {
        method: 'PUT',
        body: JSON.stringify(feedback),
      }
    );
  }

  // Steps
  async getSteps(params?: {
    active?: boolean;
    page?: number;
    limit?: number;
  }): Promise<{ steps: Step[]; pagination: any }> {
    const searchParams = new URLSearchParams();
    Object.entries(params || {}).forEach(([key, value]) => {
      if (value !== undefined) {
        searchParams.append(key, String(value));
      }
    });

    const queryString = searchParams.toString();
    return this.request<{ steps: Step[]; pagination: any }>(
      `/projects/${this.projectId}/steps${queryString ? `?${queryString}` : ''}`
    );
  }

  async getStep(id: number): Promise<{ step: Step }> {
    return this.request<{ step: Step }>(
      `/projects/${this.projectId}/steps/${id}`
    );
  }

  // Project endpoints
  async getProjects(): Promise<{ projects: any[] }> {
    return this.request<{ projects: any[] }>('/projects');
  }

  async getProject(id: string): Promise<{ project: any }> {
    return this.request<{ project: any }>(`/projects/${id}`);
  }
}

export const apiService = new ApiService();
