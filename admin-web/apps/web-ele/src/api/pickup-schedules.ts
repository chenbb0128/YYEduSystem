import { businessRequestClient } from '#/api/request';

export type PickupScheduleMode = 'school_pickup' | 'self_arrival';

export interface PickupScheduleRecord {
  id: number;
  school_id: number;
  school_class_id: number;
  care_class_id?: number;
  weekday: number;
  weekday_label: string;
  pickup_mode: PickupScheduleMode;
  teacher_user_id?: number;
  teacher_name: string;
  expected_pickup_time: string;
  effective_from: string;
  effective_to?: string;
  enabled: boolean;
  notes: string;
  school_name?: string;
  grade?: string;
  class_name?: string;
  created_at: string;
  updated_at: string;
}

interface PageResult<T> {
  items: T[];
  total: number;
}

export function getPickupSchedulesApi(params?: { date?: string }) {
  return businessRequestClient.get<PageResult<PickupScheduleRecord>>(
    '/pickup-schedules',
    { params },
  );
}

export interface PickupSchedulePayload {
  school_id: number;
  school_class_id: number;
  care_class_id?: number;
  weekday: number;
  pickup_mode: PickupScheduleMode;
  teacher_user_id?: number;
  teacher_name?: string;
  expected_pickup_time?: string;
  effective_from: string;
  effective_to?: string;
  enabled?: boolean;
  notes?: string;
}

export function createPickupScheduleApi(data: PickupSchedulePayload) {
  return businessRequestClient.post<PickupScheduleRecord>(
    '/pickup-schedules',
    data,
  );
}

export function updatePickupScheduleApi(
  id: number,
  data: PickupSchedulePayload,
) {
  return businessRequestClient.put<PickupScheduleRecord>(
    `/pickup-schedules/${id}`,
    data,
  );
}

export function generatePickupSchedulesApi(date: string) {
  return businessRequestClient.post<{
    created_operation_ids: number[];
    date: string;
    skipped_reasons: Record<number, string>;
    skipped_schedule_ids: number[];
  }>('/pickup-schedules/generate', { date });
}
