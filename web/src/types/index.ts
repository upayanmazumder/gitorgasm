export interface ApiErrorData {
  message: string;
  status?: number;
  code?: string;
  requestId?: string;
  details?: unknown;
}
