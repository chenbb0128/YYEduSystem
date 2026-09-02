import { request } from '@/services/request'

export interface TeacherAssignmentRecord {
  id: number
  teacher_user_id: number
  teacher_name: string
  username: string
  school_class_id: number
  school_id: number
  grade: string
  class_name: string
  status: 'active' | 'disabled'
}

interface ApiEnvelope<T> {
  code: number
  data: T
  message: string
}

export function getTeacherAssignments() {
  return request<ApiEnvelope<{ items: TeacherAssignmentRecord[], total: number }>>({ method: 'GET', url: '/teacher-assignments' }).then((response) => {
    if (response.code !== 0) {
      throw new Error(response.message || '负责班级加载失败')
    }
    return response.data
  })
}
