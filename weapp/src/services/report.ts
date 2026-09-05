import { request } from '@/services/request'

export type DailyExceptionCategory = 'application' | 'homework' | 'leave' | 'meal' | 'pickup' | 'student' | 'summary'
export type DailyExceptionSeverity = 'danger' | 'warning'

export interface DailyException {
  id: string
  code: string
  category: DailyExceptionCategory
  severity: DailyExceptionSeverity
  label: string
  message: string
  school_class_id?: number
  class_name?: string
  student_id?: number
  student_name?: string
  operation_id?: number
  task_id?: number
  action: string
  acknowledged?: boolean
  acknowledged_at?: string
  acknowledged_by?: string
}

export interface DailyExceptions {
  date: string
  items: DailyException[]
  counts: Record<string, number>
}

interface ApiEnvelope<T> {
  code: number
  data: T
  message: string
}

export async function getDailyExceptions(date: string, includeAcknowledged = false, filter: { school_class_id?: number, operation_id?: number } = {}) {
  const params = [`date=${encodeURIComponent(date)}`]
  if (includeAcknowledged) {
    params.push('include_acknowledged=true')
  }
  if (filter.school_class_id) {
    params.push(`school_class_id=${encodeURIComponent(filter.school_class_id)}`)
  }
  if (filter.operation_id) {
    params.push(`operation_id=${encodeURIComponent(filter.operation_id)}`)
  }
  const query = `?${params.join('&')}`
  const response = await request<ApiEnvelope<DailyExceptions>>({
    method: 'GET',
    url: `/reports/daily-exceptions${query}`,
  })
  if (response.code !== 0) {
    throw new Error(response.message || '异常数据加载失败')
  }
  return response.data
}

export async function acknowledgeDailyException(id: string, date: string, note = '') {
  const response = await request<ApiEnvelope<{ id: string, acknowledged: boolean }>>({
    method: 'POST',
    url: `/reports/daily-exceptions/${encodeURIComponent(id)}/acknowledge?date=${encodeURIComponent(date)}`,
    data: { note: note.trim() },
  })
  if (response.code !== 0) {
    throw new Error(response.message || '异常处理失败')
  }
  return response.data
}

export function dailyExceptionCategoryLabel(category: DailyExceptionCategory) {
  return ({
    application: '入班申请',
    homework: '作业',
    leave: '请假',
    meal: '餐食',
    pickup: '接送',
    student: '学生档案',
    summary: '每日总结',
  } satisfies Record<DailyExceptionCategory, string>)[category]
}
