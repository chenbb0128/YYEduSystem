import { businessRequestClient } from '#/api/request';

export type TeacherAssignmentStatus = 'active' | 'disabled';

export interface TeacherAssignmentRecord {
  id: number;
  teacher_user_id: number;
  teacher_name: string;
  username: string;
  school_class_id: number;
  school_id: number;
  grade: string;
  class_name: string;
  status: TeacherAssignmentStatus;
  created_at: string;
  updated_at: string;
}

export interface TeacherAssignmentPayload {
  teacher_user_id: number;
  school_class_id: number;
}

export interface TeacherAssignmentStatusPayload {
  status: TeacherAssignmentStatus;
}

interface PageResult<T> {
  items: T[];
  total: number;
}

export function getTeacherAssignmentsApi() {
  return businessRequestClient.get<PageResult<TeacherAssignmentRecord>>(
    '/teacher-assignments',
  );
}

export function createTeacherAssignmentApi(data: TeacherAssignmentPayload) {
  return businessRequestClient.post<TeacherAssignmentRecord>(
    '/teacher-assignments',
    data,
  );
}

export function updateTeacherAssignmentApi(
  id: number,
  data: TeacherAssignmentStatusPayload,
) {
  return businessRequestClient.put<TeacherAssignmentRecord>(
    `/teacher-assignments/${id}`,
    data,
  );
}
