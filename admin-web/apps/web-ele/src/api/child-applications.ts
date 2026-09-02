import { businessRequestClient } from '#/api/request';

export type ChildApplicationStatus =
  | 'approved'
  | 'needs_info'
  | 'pending'
  | 'rejected';

export interface StudentMatchRecord {
  id: number;
  name: string;
  guardian_phone?: string;
}

export interface ChildApplicationRecord {
  id: number;
  parent_account_id?: number;
  student_id?: number;
  student_name: string;
  school_name_input: string;
  grade_input: string;
  class_name_input: string;
  school_id?: number;
  school_class_id?: number;
  grade: string;
  class_name: string;
  guardian_name: string;
  guardian_phone: string;
  relationship: string;
  notes: string;
  status: ChildApplicationStatus;
  review_note: string;
  student_matches?: StudentMatchRecord[];
  reviewed_at?: string;
  created_at: string;
}

export function getChildApplicationsApi() {
  return businessRequestClient.get<{
    items: ChildApplicationRecord[];
    total: number;
  }>('/child-applications');
}

export interface ReviewChildApplicationPayload {
  status: Exclude<ChildApplicationStatus, 'pending'>;
  school_class_id?: number;
  student_id?: number;
  create_school_class?: boolean;
  review_note?: string;
}

export function reviewChildApplicationApi(
  id: number,
  data: ReviewChildApplicationPayload,
) {
  return businessRequestClient.post<ChildApplicationRecord>(
    `/child-applications/${id}/review`,
    data,
  );
}
