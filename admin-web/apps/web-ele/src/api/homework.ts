import { businessRequestClient } from '#/api/request';

export type HomeworkTaskStatus = 'active' | 'cancelled';
export type HomeworkStudentStatus =
  | 'completed'
  | 'incomplete'
  | 'not_submitted'
  | 'pending';

export interface HomeworkTaskRecord {
  id: number;
  homework_date: string;
  school_id: number;
  school_class_id: number;
  subject: string;
  content: string;
  attachment_urls: string[];
  created_by_user_id?: number;
  creator_name: string;
  status: HomeworkTaskStatus;
  created_at: string;
  updated_at: string;
}

export interface HomeworkTaskStudentRecord {
  id: number;
  task_id: number;
  student_id: number;
  student_name: string;
  status: HomeworkStudentStatus;
  correction_note: string;
  reviewed_by_user_id?: number;
  reviewed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface HomeworkTaskPayload {
  homework_date: string;
  school_class_id: number;
  subject: string;
  content: string;
  attachment_urls?: string[];
  student_ids?: number[];
}

interface PageResult<T> {
  items: T[];
  total: number;
}

export function getHomeworkTasksApi(params?: {
  date?: string;
  school_class_id?: number;
  status?: HomeworkTaskStatus;
}) {
  return businessRequestClient.get<PageResult<HomeworkTaskRecord>>(
    '/homework-tasks',
    { params },
  );
}

export function createHomeworkTaskApi(data: HomeworkTaskPayload) {
  return businessRequestClient.post<HomeworkTaskRecord>(
    '/homework-tasks',
    data,
  );
}

export function uploadHomeworkPhotoApi(
  file: File,
  params: { task_id?: number } = {},
) {
  const formData = new FormData();
  formData.append('file', file);
  if (params.task_id) {
    formData.append('task_id', String(params.task_id));
  }
  return businessRequestClient.post<{
    content_type: string;
    key?: string;
    sha256?: string;
    size: number;
    url: string;
  }>('/uploads/homework', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  });
}

export function getHomeworkTaskStudentsApi(taskId: number) {
  return businessRequestClient.get<PageResult<HomeworkTaskStudentRecord>>(
    `/homework-tasks/${taskId}/students`,
  );
}

export function reviewHomeworkStudentApi(
  taskId: number,
  studentId: number,
  data: {
    correction_note?: string;
    status: Exclude<HomeworkStudentStatus, 'pending'>;
  },
) {
  return businessRequestClient.post<HomeworkTaskStudentRecord>(
    `/homework-tasks/${taskId}/students/${studentId}/review`,
    data,
  );
}
