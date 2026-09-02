import { businessRequestClient } from '#/api/request';

export type PickupOperationStatus =
  | 'cancelled'
  | 'confirmed'
  | 'draft'
  | 'finished'
  | 'started';
export type PickupMemberStatus =
  | 'abnormal'
  | 'absent'
  | 'arrived'
  | 'leave'
  | 'left'
  | 'midway_left'
  | 'not_arrived'
  | 'parent_picked_up'
  | 'picked_up'
  | 'planned'
  | 'self_arrived';

export interface PickupOperationRecord {
  id: number;
  operation_date: string;
  pickup_mode: 'school_pickup' | 'self_arrival';
  school_id: number;
  school_class_id: number;
  care_class_id?: number;
  teacher_user_id?: number;
  teacher_name: string;
  status: PickupOperationStatus;
  started_at?: string;
  finished_at?: string;
  confirmed_at?: string;
  confirmed_by_name?: string;
  executing_teacher_user_id?: number;
  executing_teacher_name?: string;
  teacher_role?: 'collaborator' | 'lead' | 'substitute';
  expected_pickup_time?: string;
  notes: string;
}

export interface PickupOperationStudentRecord {
  id: number;
  operation_id: number;
  student_id: number;
  student_name: string;
  status: PickupMemberStatus;
  photo_url?: string;
  checked_at?: string;
  note: string;
  is_temporary: boolean;
  profile_pending: boolean;
  pickup_mode?: 'parent_picked_up' | 'school_pickup' | 'self_arrival';
}

export interface PickupEventRecord {
  id: number;
  operation_student_id: number;
  student_id: number;
  event_type: 'correction' | PickupMemberStatus;
  event_at: string;
  operator_name: string;
  photo_url?: string;
  note: string;
}

export interface PickupHandoffRecord {
  id: number;
  operation_id: number;
  from_teacher_user_id?: number;
  from_teacher_name: string;
  to_teacher_user_id: number;
  to_teacher_name: string;
  teacher_role: 'collaborator' | 'lead' | 'substitute';
  note: string;
  handoff_at: string;
  created_by_name: string;
}

export interface PickupHandoffTeacherRecord {
  teacher_user_id: number;
  teacher_name: string;
  username: string;
}

export interface PickupNotificationRecord {
  id: number;
  student_id: number;
  operation_id?: number;
  event_id?: number;
  recipient_type: 'parent';
  kind: string;
  title: string;
  content: string;
  status: 'failed' | 'pending' | 'sent';
  created_at: string;
}

export interface PickupPageResult<T> {
  items: T[];
  total: number;
}

export interface CreatePickupOperationPayload {
  care_class_id?: number;
  notes: string;
  operation_date: string;
  pickup_mode: 'school_pickup' | 'self_arrival';
  school_class_id: number;
  student_ids?: number[];
  teacher_user_id?: number;
  teacher_name: string;
}

export function getPickupOperationsApi(params?: {
  date?: string;
  status?: PickupOperationStatus;
}) {
  return businessRequestClient.get<PickupPageResult<PickupOperationRecord>>(
    '/pickup-operations',
    { params },
  );
}

export function createPickupOperationApi(data: CreatePickupOperationPayload) {
  return businessRequestClient.post<PickupOperationRecord>(
    '/pickup-operations',
    data,
  );
}

export function getPickupOperationStudentsApi(operationId: number) {
  return businessRequestClient.get<
    PickupPageResult<PickupOperationStudentRecord>
  >(`/pickup-operations/${operationId}/students`);
}

export function startPickupOperationApi(operationId: number) {
  return businessRequestClient.post<PickupOperationRecord>(
    `/pickup-operations/${operationId}/start`,
  );
}

export function confirmPickupOperationApi(
  operationId: number,
  data?: {
    executing_teacher_name?: string;
    executing_teacher_user_id?: number;
    expected_pickup_time?: string;
    notes?: string;
    teacher_role?: 'collaborator' | 'lead' | 'substitute';
  },
) {
  return businessRequestClient.post<PickupOperationRecord>(
    `/pickup-operations/${operationId}/confirm`,
    data || {},
  );
}

export function finishPickupOperationApi(operationId: number) {
  return businessRequestClient.post<PickupOperationRecord>(
    `/pickup-operations/${operationId}/finish`,
  );
}

export function markPickupStudentApi(
  operationId: number,
  studentId: number,
  data: {
    note?: string;
    operator_name?: string;
    photo_url?: string;
    status: Exclude<PickupMemberStatus, 'planned'>;
  },
) {
  return businessRequestClient.post<PickupOperationStudentRecord>(
    `/pickup-operations/${operationId}/students/${studentId}/status`,
    data,
  );
}

export function getPickupEventsApi(operationId: number) {
  return businessRequestClient.get<PickupPageResult<PickupEventRecord>>(
    `/pickup-operations/${operationId}/events`,
  );
}

export function getPickupHandoffsApi(operationId: number) {
  return businessRequestClient.get<PickupPageResult<PickupHandoffRecord>>(
    `/pickup-operations/${operationId}/handoffs`,
  );
}

export function getPickupHandoffTeachersApi(operationId: number) {
  return businessRequestClient.get<
    PickupPageResult<PickupHandoffTeacherRecord>
  >(`/pickup-operations/${operationId}/handoff-teachers`);
}

export function handoverPickupOperationApi(
  operationId: number,
  data: {
    note?: string;
    teacher_role?: 'collaborator' | 'lead' | 'substitute';
    to_teacher_name?: string;
    to_teacher_user_id: number;
  },
) {
  return businessRequestClient.post<PickupOperationRecord>(
    `/pickup-operations/${operationId}/handover`,
    data,
  );
}

export function correctPickupEventApi(
  operationId: number,
  eventId: number,
  data: { reason: string; status: Exclude<PickupMemberStatus, 'planned'> },
) {
  return businessRequestClient.post<PickupOperationStudentRecord>(
    `/pickup-operations/${operationId}/events/${eventId}/correct`,
    data,
  );
}

export function getPickupNotificationsApi() {
  return businessRequestClient.get<PickupPageResult<PickupNotificationRecord>>(
    '/notifications',
  );
}

export function uploadPickupPhotoApi(
  file: File,
  params: { operation_id?: number } = {},
) {
  const formData = new FormData();
  formData.append('file', file);
  if (params.operation_id) {
    formData.append('operation_id', String(params.operation_id));
  }
  return businessRequestClient.post<{
    content_type: string;
    key?: string;
    sha256?: string;
    size: number;
    url: string;
  }>('/uploads/pickup', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  });
}
