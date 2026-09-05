import { mediaURL } from '@/services/media'
import { request } from '@/services/request'

export type WrongQuestionStatus = 'active' | 'archived' | 'mastered'

export interface WrongQuestion {
  id: number
  student_id: number
  student_name: string
  subject: string
  question_text: string
  answer_text: string
  explanation: string
  knowledge_point: string
  source_image_url: string
  source_homework_task_id?: number
  teacher_note: string
  status: WrongQuestionStatus
  created_by_user_id?: number
  created_by_name: string
  last_reviewed_at?: string
  created_at: string
  updated_at: string
}

export interface ExtractedWrongQuestion {
  temp_id: string
  subject: string
  question_text: string
  answer_text: string
  explanation: string
  knowledge_point: string
  confidence: number
}

export interface WrongPaper {
  id: number
  student_id: number
  student_name: string
  title: string
  source: 'parent' | 'system' | 'teacher'
  status: 'archived' | 'assigned' | 'generated'
  generated_by_type: 'parent' | 'staff' | 'system'
  generated_by_user_id?: number
  question_count: number
  questions?: WrongQuestion[]
  created_at: string
  updated_at: string
}

export interface PageResult<T> {
  items: T[]
  total: number
}

interface ApiEnvelope<T> {
  code: number
  data: T
  message: string
}

async function wrongbookRequest<T>(url: string, method: 'GET' | 'POST' | 'PUT' = 'GET', data?: unknown, params?: Record<string, string>) {
  const response = await request<ApiEnvelope<T>>({ method, url, data, params })
  if (response.code !== 0) {
    throw new Error(response.message || '错题集请求失败')
  }
  return response.data
}

export function extractWrongQuestions(data: { image_url?: string, source_text?: string, subject?: string }) {
  return wrongbookRequest<{ image_url: string, items: ExtractedWrongQuestion[], mocked: boolean, total: number }>('/wrong-questions/extract', 'POST', data)
}

export function getWrongQuestions(params: { keyword?: string, status?: WrongQuestionStatus | '', student_id?: number, subject?: string } = {}) {
  const query: Record<string, string> = {}
  if (params.student_id) {
    query.student_id = String(params.student_id)
  }
  if (params.subject) {
    query.subject = params.subject
  }
  if (params.status) {
    query.status = params.status
  }
  if (params.keyword) {
    query.keyword = params.keyword
  }
  return wrongbookRequest<PageResult<WrongQuestion>>('/wrong-questions', 'GET', undefined, query)
}

export function createWrongQuestions(items: Array<{
  answer_text?: string
  explanation?: string
  knowledge_point?: string
  question_text: string
  source_homework_task_id?: number
  source_image_url?: string
  student_id: number
  subject: string
  teacher_note?: string
}>) {
  return wrongbookRequest<PageResult<WrongQuestion>>('/wrong-questions/bulk', 'POST', { items })
}

export function updateWrongQuestion(id: number, data: {
  answer_text?: string
  explanation?: string
  knowledge_point?: string
  question_text: string
  status: WrongQuestionStatus
  subject: string
  teacher_note?: string
}) {
  return wrongbookRequest<WrongQuestion>(`/wrong-questions/${id}`, 'PUT', data)
}

export function getWrongPapers(params: { student_id?: number } = {}) {
  const query: Record<string, string> = {}
  if (params.student_id) {
    query.student_id = String(params.student_id)
  }
  return wrongbookRequest<PageResult<WrongPaper>>('/wrong-papers', 'GET', undefined, query)
}

export function getWrongPaper(id: number) {
  return wrongbookRequest<WrongPaper>(`/wrong-papers/${id}`)
}

export function createWrongPaper(data: { question_ids: number[], student_id: number, title?: string }) {
  return wrongbookRequest<WrongPaper>('/wrong-papers', 'POST', data)
}

export function getParentWrongQuestions(studentID: number, params: { keyword?: string, status?: WrongQuestionStatus | '', subject?: string } = {}) {
  const query: Record<string, string> = {}
  if (params.subject) {
    query.subject = params.subject
  }
  if (params.status) {
    query.status = params.status
  }
  if (params.keyword) {
    query.keyword = params.keyword
  }
  return wrongbookRequest<PageResult<WrongQuestion>>(`/parent/students/${studentID}/wrong-questions`, 'GET', undefined, query)
}

export function getParentWrongPapers(studentID: number) {
  return wrongbookRequest<PageResult<WrongPaper>>(`/parent/students/${studentID}/wrong-papers`)
}

export function getParentWrongPaper(studentID: number, id: number) {
  return wrongbookRequest<WrongPaper>(`/parent/students/${studentID}/wrong-papers/${id}`)
}

export function createParentWrongPaper(studentID: number, data: { question_ids: number[], title?: string }) {
  return wrongbookRequest<WrongPaper>(`/parent/students/${studentID}/wrong-papers`, 'POST', data)
}

export function wrongQuestionStatusLabel(status: WrongQuestionStatus) {
  return ({ active: '待复习', archived: '已归档', mastered: '已掌握' })[status]
}

export function wrongQuestionPhotoURL(path?: string) {
  return mediaURL(path)
}
