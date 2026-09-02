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

export function getDailyOverviewApi(params?: { date?: string }) {
  return businessRequestClient.get<DailyOverviewRecord>(
    '/reports/daily-overview',
    { params },
  );
}
