import { businessRequestClient } from '#/api/request';

export type LeaveRequestStatus =
  | 'approved'
  | 'cancelled'
  | 'pending'
  | 'rejected';

export interface LeaveRequestRecord {
  id: number;
  student_id: number;
  parent_account_id?: number;
  submitted_by_type: 'parent' | 'teacher';
  leave_date: string;
  reason: string;
  status: LeaveRequestStatus;
  teacher_note: string;
  reviewed_at?: string;
  created_at: string;
}

interface PageResult<T> {
  items: T[];
  total: number;
}

export function getLeaveRequestsApi() {
  return businessRequestClient.get<PageResult<LeaveRequestRecord>>(
    '/leave-requests',
  );
}

export function reviewLeaveRequestApi(
  id: number,
  data: { status: 'approved' | 'rejected'; teacher_note?: string },
) {
  return businessRequestClient.post<LeaveRequestRecord>(
    `/leave-requests/${id}/review`,
    data,
  );
}
