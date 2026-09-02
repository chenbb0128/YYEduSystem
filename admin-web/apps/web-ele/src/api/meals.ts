import { businessRequestClient } from '#/api/request';

export interface MealPlanRecord {
  id: number;
  meal_date: string;
  menu_text: string;
  photo_url?: string;
  adjustment_note: string;
  created_by_name: string;
  status: 'active' | 'closed';
  created_at: string;
  updated_at: string;
}

export interface DietNoteRecord {
  id: number;
  student_id: number;
  note: string;
  updated_by_name: string;
  updated_at: string;
}

export interface DietNoteChangeRequestRecord {
  id: number;
  student_id: number;
  parent_account_id?: number;
  current_note: string;
  requested_note: string;
  status: 'approved' | 'pending' | 'rejected';
  review_note: string;
  reviewed_at?: string;
  created_at: string;
  updated_at: string;
}

interface PageResult<T> {
  items: T[];
  total: number;
}

export function getMealPlansApi(params?: { from?: string; to?: string }) {
  return businessRequestClient.get<PageResult<MealPlanRecord>>('/meals', {
    params,
  });
}

export function upsertMealPlanApi(data: {
  adjustment_note?: string;
  meal_date: string;
  menu_text: string;
  photo_url?: string;
}) {
  return businessRequestClient.post<MealPlanRecord>('/meals', data);
}

export function copyMealPlanApi(data: {
  source_date: string;
  target_date: string;
}) {
  return businessRequestClient.post<MealPlanRecord>('/meals/copy', data);
}

export function uploadMealPhotoApi(
  file: File,
  params: { meal_plan_id?: number } = {},
) {
  const formData = new FormData();
  formData.append('file', file);
  if (params.meal_plan_id) {
    formData.append('meal_plan_id', String(params.meal_plan_id));
  }
  return businessRequestClient.post<{
    content_type: string;
    key?: string;
    sha256?: string;
    size: number;
    url: string;
  }>('/uploads/meals', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  });
}

export function getDietNotesApi(params?: { student_id?: number }) {
  return businessRequestClient.get<PageResult<DietNoteRecord>>(
    '/meal-diet-notes',
    { params },
  );
}

export function updateDietNoteApi(studentId: number, data: { note: string }) {
  return businessRequestClient.put<DietNoteRecord>(
    `/students/${studentId}/diet-note`,
    data,
  );
}

export function getDietNoteChangeRequestsApi(params?: {
  status?: DietNoteChangeRequestRecord['status'];
  student_id?: number;
}) {
  return businessRequestClient.get<PageResult<DietNoteChangeRequestRecord>>(
    '/diet-note-change-requests',
    { params },
  );
}

export function reviewDietNoteChangeRequestApi(
  id: number,
  data: { review_note?: string; status: 'approved' | 'rejected' },
) {
  return businessRequestClient.post<DietNoteChangeRequestRecord>(
    `/diet-note-change-requests/${id}/review`,
    data,
  );
}
