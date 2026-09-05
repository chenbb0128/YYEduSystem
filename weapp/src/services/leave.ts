import { request } from '@/services/request'

export interface TeacherLeaveRequest {
  id: number
  student_id: number
  parent_account_id?: number
  submitted_by_type: string
  leave_date: string
  reason: string
  status: string
  teacher_note?: string
  reviewed_at?: string
  created_at?: string
}

interface ApiEnvelope<T> {
  code: number
  data: T
  message: string
}

export function createTeacherLeaveRequest(data: { student_id: number, leave_date: string, reason: string }) {
  return request<ApiEnvelope<TeacherLeaveRequest>>({ method: 'POST', url: '/leave-requests/teacher', data }).then((response) => {
    if (response.code !== 0) {
      throw new Error(response.message || '口头请假登记失败')
    }
    return response.data
  })
}

export async function getTeacherLeaveRequests(date: string, status = 'pending') {
  const response = await request<ApiEnvelope<{ items: TeacherLeaveRequest[], total: number }>>({ method: 'GET', url: `/leave-requests?date=${encodeURIComponent(date)}&status=${encodeURIComponent(status)}` })
  if (response.code !== 0) {
    throw new Error(response.message || '请假申请加载失败')
  }
  return response.data
}

export async function reviewTeacherLeaveRequest(id: number, data: { status: 'approved' | 'rejected', teacher_note?: string }) {
  const response = await request<ApiEnvelope<TeacherLeaveRequest>>({ method: 'POST', url: `/leave-requests/${id}/review`, data })
  if (response.code !== 0) {
    throw new Error(response.message || '请假审核失败')
  }
  return response.data
}
