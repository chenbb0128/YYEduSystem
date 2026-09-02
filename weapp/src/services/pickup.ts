import { appEnv } from '@/config/env'
import { request } from '@/services/request'
import { getAccessToken } from '@/utils/storage'

export type PickupOperationStatus = 'cancelled' | 'confirmed' | 'draft' | 'finished' | 'started'
export type PickupMemberStatus = 'absent' | 'abnormal' | 'arrived' | 'leave' | 'left' | 'midway_left' | 'not_arrived' | 'parent_picked_up' | 'picked_up' | 'planned' | 'self_arrived'

export interface PickupOperation {
  id: number
  operation_date: string
  pickup_mode: 'school_pickup' | 'self_arrival'
  school_id: number
  school_class_id: number
  care_class_id?: number
  teacher_name: string
  confirmed_at?: string
  confirmed_by_name?: string
  executing_teacher_user_id?: number
  executing_teacher_name?: string
  teacher_role?: 'lead' | 'collaborator' | 'substitute'
  expected_pickup_time?: string
  status: PickupOperationStatus
  notes: string
}

export interface PickupOperationStudent {
  id: number
  operation_id: number
  student_id: number
  student_name: string
  status: PickupMemberStatus
  photo_url?: string
  checked_at?: string
  note: string
  is_temporary: boolean
  profile_pending: boolean
  pickup_mode?: 'school_pickup' | 'self_arrival' | 'parent_picked_up'
}

export interface PickupEvent {
  id: number
  operation_student_id: number
  student_id: number
  event_type: PickupMemberStatus | 'correction'
  event_at: string
  operator_name: string
  photo_url?: string
  note: string
}

export interface PickupHandoff {
  id: number
  operation_id: number
  from_teacher_user_id?: number
  from_teacher_name: string
  to_teacher_user_id: number
  to_teacher_name: string
  teacher_role: 'lead' | 'collaborator' | 'substitute'
  note: string
  handoff_at: string
  created_by_name: string
}

export interface PickupHandoffTeacher {
  teacher_user_id: number
  teacher_name: string
  username: string
}

export interface CompleteTemporaryPickupStudentProfilePayload {
  guardian_phone?: string
  gender?: string
  student_no?: string
  emergency_contact?: string
  emergency_phone?: string
  notes?: string
}

export interface CreatePickupOperationPayload {
  operation_date: string
  school_class_id: number
  pickup_mode?: 'school_pickup' | 'self_arrival'
  care_class_id?: number
  student_ids?: number[]
  notes?: string
}

export interface PickupWorkbenchOperation {
  operation: PickupOperation
  students: PickupOperationStudent[]
  counts: Record<string, number>
}

export interface PickupWorkbench {
  date: string
  operations: PickupWorkbenchOperation[]
  totals: Record<string, number>
  alerts: Array<{ kind: string, operation_id: number, student_id?: number, student_name?: string, message: string }>
}

export interface PickupCloseCheck {
  operation_id: number
  can_finish: boolean
  pending: PickupOperationStudent[]
  exceptions: Array<{ kind: string, operation_id: number, student_id?: number, student_name?: string, message: string }>
  pending_photo_count: number
  profile_pending_count: number
}

export type PickupChangeRequestStatus = 'approved' | 'pending' | 'rejected'

export interface PickupChangeRequest {
  id: number
  student_id: number
  student_name: string
  operation_id?: number
  change_date: string
  requested_status: Exclude<PickupMemberStatus, 'planned'>
  note: string
  submitted_by: string
  status: PickupChangeRequestStatus
  reviewed_at?: string
  review_note: string
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

async function pickupRequest<T>(url: string, method: 'GET' | 'POST' = 'GET', data?: unknown) {
  const response = await request<ApiEnvelope<T>>({ method, url, data })
  if (response.code !== 0) {
    throw new Error(response.message || '请求失败')
  }
  return response.data
}

export function getToday() {
  const now = new Date()
  const month = `${now.getMonth() + 1}`.padStart(2, '0')
  const day = `${now.getDate()}`.padStart(2, '0')
  return `${now.getFullYear()}-${month}-${day}`
}

export async function getPickupOperations(date = getToday()) {
  return pickupRequest<PageResult<PickupOperation>>(`/pickup-operations?date=${date}`)
}

export async function getPickupWorkbench(date = getToday()) {
  return pickupRequest<PickupWorkbench>(`/pickup-workbench?date=${encodeURIComponent(date)}`)
}

export async function getPickupChangeRequests(params: { date?: string, status?: PickupChangeRequestStatus } = {}) {
  const query: string[] = []
  if (params.date) {
    query.push(`date=${encodeURIComponent(params.date)}`)
  }
  if (params.status) {
    query.push(`status=${encodeURIComponent(params.status)}`)
  }
  const suffix = query.length ? `?${query.join('&')}` : ''
  return pickupRequest<PageResult<PickupChangeRequest>>(`/pickup-change-requests${suffix}`)
}

export async function reviewPickupChangeRequest(id: number, data: { status: 'approved' | 'rejected', review_note?: string }) {
  return pickupRequest<PickupChangeRequest>(`/pickup-change-requests/${id}/review`, 'POST', data)
}

export async function createPickupOperation(data: CreatePickupOperationPayload) {
  return pickupRequest<PickupOperation>('/pickup-operations', 'POST', data)
}

export async function getPickupOperationStudents(operationId: number) {
  return pickupRequest<PageResult<PickupOperationStudent>>(`/pickup-operations/${operationId}/students`)
}

export async function getPickupEvents(operationId: number) {
  return pickupRequest<{ items: PickupEvent[], total: number }>(`/pickup-operations/${operationId}/events`)
}

export async function getPickupHandoffs(operationId: number) {
  return pickupRequest<{ items: PickupHandoff[], total: number }>(`/pickup-operations/${operationId}/handoffs`)
}

export async function getPickupHandoffTeachers(operationId: number) {
  return pickupRequest<{ items: PickupHandoffTeacher[], total: number }>(`/pickup-operations/${operationId}/handoff-teachers`)
}

export async function handoverPickupOperation(operationId: number, data: { to_teacher_user_id: number, to_teacher_name?: string, teacher_role?: 'lead' | 'collaborator' | 'substitute', note?: string }) {
  return pickupRequest<PickupOperation>(`/pickup-operations/${operationId}/handover`, 'POST', data)
}

export async function correctPickupEvent(operationId: number, eventId: number, status: Exclude<PickupMemberStatus, 'planned'>, reason: string) {
  return pickupRequest<PickupOperationStudent>(`/pickup-operations/${operationId}/events/${eventId}/correct`, 'POST', { status, reason })
}

export async function getPickupCloseCheck(operationId: number) {
  return pickupRequest<PickupCloseCheck>(`/pickup-operations/${operationId}/close-check`)
}

export async function startPickupOperation(operationId: number) {
  return pickupRequest<PickupOperation>(`/pickup-operations/${operationId}/start`, 'POST')
}

export async function confirmPickupOperation(operationId: number, data: { executing_teacher_user_id?: number, executing_teacher_name?: string, teacher_role?: string, expected_pickup_time?: string, notes?: string } = {}) {
  return pickupRequest<PickupOperation>(`/pickup-operations/${operationId}/confirm`, 'POST', data)
}

export async function finishPickupOperation(operationId: number) {
  return pickupRequest<PickupOperation>(`/pickup-operations/${operationId}/finish`, 'POST')
}

export async function markPickupStudent(operationId: number, studentId: number, status: Exclude<PickupMemberStatus, 'planned'>, photoUrl = '', note = '') {
  return pickupRequest<PickupOperationStudent>(`/pickup-operations/${operationId}/students/${studentId}/status`, 'POST', { status, photo_url: photoUrl, operator_name: '老师', note })
}

export async function addTemporaryPickupStudent(operationId: number, data: { name: string, guardian_phone?: string, gender?: string, student_no?: string, pickup_mode?: string, note?: string }) {
  return pickupRequest<PickupOperationStudent>(`/pickup-operations/${operationId}/students`, 'POST', data)
}

export async function completeTemporaryPickupStudentProfile(operationId: number, studentId: number, data: CompleteTemporaryPickupStudentProfilePayload) {
  return pickupRequest<{ student: unknown, operation_student: PickupOperationStudent }>(`/pickup-operations/${operationId}/students/${studentId}/profile`, 'POST', data)
}

export function uploadPickupPhoto(filePath: string, formData: { operation_id?: number } = {}) {
  return new Promise<string>((resolve, reject) => {
    if (typeof wx === 'undefined') {
      reject(new Error('当前环境不支持照片上传'))
      return
    }
    const uploadFields: Record<string, string> = {}
    for (const [key, value] of Object.entries(formData)) {
      if (typeof value === 'number' && value > 0) {
        uploadFields[key] = String(value)
      }
    }
    wx.uploadFile({
      filePath,
      name: 'file',
      formData: uploadFields,
      url: `${appEnv.apiBaseUrl}/uploads/pickup`,
      header: getAccessToken() ? { Authorization: `Bearer ${getAccessToken()}` } : undefined,
      success: (result) => {
        try {
          const response = JSON.parse(result.data) as ApiEnvelope<{ url: string }>
          if (result.statusCode >= 200 && result.statusCode < 300 && response.code === 0 && response.data.url) {
            resolve(response.data.url)
            return
          }
          reject(new Error(response.message || '照片上传失败'))
        }
        catch (error) {
          reject(error)
        }
      },
      fail: reject,
    })
  })
}

export function pickupStatusLabel(status: PickupMemberStatus) {
  return ({
    absent: '未到',
    leave: '请假',
    parent_picked_up: '家长接走',
    picked_up: '校门口接到',
    planned: '待确认',
    self_arrived: '自行到班',
    arrived: '已到托管班',
    not_arrived: '到班异常',
    left: '已离班',
    midway_left: '中途离班',
    abnormal: '其他异常',
  })[status]
}

export function pickupOperationStatusLabel(status: PickupOperationStatus) {
  return ({ cancelled: '已取消', confirmed: '已确认', draft: '待确认', finished: '已完成', started: '接送中' })[status]
}
