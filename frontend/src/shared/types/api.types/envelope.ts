// ─── Envelope pattern (matches backend) ───

export type ApiResponse<T> = Record<string, T>;

export interface PaginatedResponse<T> {
  data: T[];
  metadata: {
    next_cursor?: string;
    has_more: boolean;
    total_count?: number;
  };
}

export interface ErrorResponse {
  error: string | { message: string; details?: Record<string, string> };
}
