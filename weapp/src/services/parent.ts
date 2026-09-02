import { mediaURL } from '@/services/media'
import { request } from '@/services/request'

export type LeaveStatus = 'approved' | 'cancelled' | 'pending' | 'rejected'

export interface ParentChild {
  id: number
  student_id: number
  student_name: string
  school_class_id: number
  care_class_id?: number
  school_name?: string
  grade?: string
  class_name?: string
  care_class_name?: string
  relationship: string
  is_primary: boolean
}

export interface ParentAccount {
  id: number
  nickname: string
  avatar: string
  status: 'active' | 'disabled'
}

export interface ParentMe {
  account: ParentAccount
  children: ParentChild[]
}

export interface ParentPickupEvent {
  id: number
  operation_id: number
  operation_student_id: number
  student_id: number
  event_type: 'absent' | 'abnormal' | 'arrived' | 'leave' | 'left' | 'midway_left' | 'not_arrived' | 'parent_picked_up' | 'picked_up' | 'self_arrived'
  event_at: string
  operator_name: string
  photo_url?: string
  note: string
}

export interface ParentNotification {
  id: number
  student_id: number
  operation_id?: number
  event_id?: number
  kind: string
  title: string
  content: string
  attachment_urls: string[]
  status: string
  read_at?: string
  created_at: string
}

export interface ParentPickupToday {
  operation_id: number
  operation_date: string
  school_class_id: number
  school_name?: string
  grade?: string
  class_name?: string
  pickup_mode: string
  status: string
  teacher_name: string
  teacher_role: string
  expected_pickup_time?: string
  student_status: string
  photo_url?: string
  profile_pending: boolean
}

export interface LeaveRequest {
  id: number
  student_id: number
  parent_account_id?: number
  submitted_by_type: string
  leave_date: string
  reason: string
  status: LeaveStatus
  teacher_note: string
  reviewed_at?: string
  created_at: string
}

export interface ParentHomework {
  task_id: number
  homework_date: string
  school_id: number
  school_class_id: number
  student_id: number
  student_name: string
  subject: string
  content: string
  status: 'completed' | 'incomplete' | 'not_submitted' | 'pending'
  correction_note: string
  creator_name: string
  attachment_urls: string[]
  reviewed_at?: string
  created_at: string
}

interface ApiEnvelope<T> {
  code: number
  data: T
  message: string
}

interface PageResult<T> {
  items: T[]
  total: number
}

export interface ParentNotificationPage extends PageResult<ParentNotification> {
  unread: number
  next_cursor: number
}

async function parentRequest<T>(url: string, method: 'GET' | 'POST' | 'PUT' = 'GET', data?: unknown, params?: Record<string, string>) {
  const response = await request<ApiEnvelope<T>>({
    method,
    url,
    data,
    params,
  })
  if (response.code !== 0) {
    throw new Error(response.message || '请求失败')
  }
  return response.data
}

export function getParentMe() {
  return parentRequest<ParentMe>('/parent/me')
}

export function getParentPickupEvents(studentID: number, date?: string) {
  return parentRequest<PageResult<ParentPickupEvent>>(`/parent/students/${studentID}/pickup-events`, 'GET', undefined, date ? { date } : undefined)
}

export function getParentPickupToday(studentID: number, date?: string) {
  return parentRequest<ParentPickupToday | null>(`/parent/students/${studentID}/pickup-today`, 'GET', undefined, date ? { date } : undefined)
}

export function getParentNotifications(params: { limit?: number, cursor?: number } = {}) {
  const query: Record<string, string> = {}
  if (params.limit) {
    query.limit = String(params.limit)
  }
  if (params.cursor) {
    query.cursor = String(params.cursor)
  }
  return parentRequest<ParentNotificationPage>('/parent/notifications', 'GET', undefined, query)
}

export function markParentNotificationRead(notificationID: number) {
  return parentRequest<{ id: number, read: boolean }>(`/parent/notifications/${notificationID}/read`, 'POST', {})
}

export function getParentLeaveRequests() {
  return parentRequest<PageResult<LeaveRequest>>('/parent/leave-requests')
}

export function getParentHomework(studentID: number, date?: string) {
  return parentRequest<PageResult<ParentHomework>>(`/parent/students/${studentID}/homework`, 'GET', undefined, date ? { date } : undefined)
}

export function createParentLeaveRequest(studentID: number, data: { leave_date: string, reason: string }) {
  return parentRequest<LeaveRequest>(`/parent/students/${studentID}/leave-requests`, 'POST', data)
}

export function updateParentLeaveRequest(leaveID: number, data: { leave_date: string, reason: string }) {
  return parentRequest<LeaveRequest>(`/parent/leave-requests/${leaveID}`, 'PUT', data)
}

export function cancelParentLeaveRequest(leaveID: number) {
  return parentRequest<LeaveRequest>(`/parent/leave-requests/${leaveID}/cancel`, 'POST', {})
}

export function createParentPickupChange(studentID: number, data: { change_date: string, requested_status: 'parent_picked_up' | 'self_arrived' | 'leave' | 'absent', note: string }) {
  return parentRequest<{ id: number, status: string }>(`/parent/students/${studentID}/pickup-changes`, 'POST', data)
}

export function parentPhotoURL(path?: string) {
  return mediaURL(path)
}

export function leaveStatusLabel(status: LeaveStatus) {
  return ({ approved: '已同意', cancelled: '已撤销', pending: '待确认', rejected: '未同意' })[status]
}
