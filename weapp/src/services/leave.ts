import { request } from '@/services/request'

export interface TeacherLeaveRequest {
  id: number
  student_id: number
  leave_date: string
  reason: string
  status: string
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
