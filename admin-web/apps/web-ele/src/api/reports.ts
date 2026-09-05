import { businessRequestClient } from '#/api/request';

export interface DailyOverviewRecord {
  date: string;
  pickup: {
    operations: number;
    photo_missing: number;
    resolved: number;
    statuses: Record<string, number>;
    students: number;
  };
  homework: {
    completed: number;
    incomplete: number;
    not_submitted: number;
    statuses: Record<string, number>;
    students: number;
    tasks: number;
  };
  meal_plans: number;
  meal_recorded: boolean;
  pending_applications: number;
  pending_leave_requests: number;
  summary_status?: string;
  anomalies: Array<{ code: string; count: number; label: string }>;
  classes: Array<{
    abnormal: number;
    class_name?: string;
    operations: number;
    resolved: number;
    school_class_id: number;
    students: number;
  }>;
}

export type DailyExceptionCategory =
  | 'application'
  | 'homework'
  | 'leave'
  | 'meal'
  | 'pickup'
  | 'student'
  | 'summary';

export interface DailyExceptionRecord {
  acknowledged?: boolean;
  acknowledged_at?: string;
  acknowledged_by?: string;
  action: string;
  category: DailyExceptionCategory;
  class_name?: string;
  code: string;
  id: string;
  label: string;
  message: string;
  operation_id?: number;
  school_class_id?: number;
  student_id?: number;
  student_name?: string;
  task_id?: number;
  severity: 'danger' | 'warning';
}

export interface DailyExceptionsRecord {
  counts: Record<string, number>;
  date: string;
  items: DailyExceptionRecord[];
}

export function getDailyOverviewApi(params?: { date?: string }) {
  return businessRequestClient.get<DailyOverviewRecord>(
    '/reports/daily-overview',
    { params },
  );
}

export function getDailyExceptionsApi(params?: {
  date?: string;
  include_acknowledged?: boolean;
}) {
  return businessRequestClient.get<DailyExceptionsRecord>(
    '/reports/daily-exceptions',
    { params },
  );
}

export function acknowledgeDailyExceptionApi(
  id: string,
  params: { date?: string; note?: string },
) {
  return businessRequestClient.post<{ acknowledged: boolean; id: string }>(
    `/reports/daily-exceptions/${encodeURIComponent(id)}/acknowledge`,
    { note: params.note?.trim() || '' },
    { params: { date: params.date } },
  );
}
