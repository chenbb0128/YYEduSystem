import { request } from '@/services/request'

export type ChildApplicationStatus = 'approved' | 'needs_info' | 'pending' | 'rejected'

export interface StudentMatch {
  id: number
  name: string
  guardian_phone?: string
}

export interface ChildApplication {
  id: number
  parent_account_id?: number
  student_id?: number
  student_name: string
  school_name_input: string
  grade_input: string
  class_name_input: string
  school_id?: number
  school_class_id?: number
  grade: string
  class_name: string
  guardian_name: string
  guardian_phone: string
  relationship: string
  notes: string
  status: ChildApplicationStatus
  review_note: string
  student_matches?: StudentMatch[]
  reviewed_at?: string
  created_at: string
}

export interface CreateChildApplicationPayload {
  student_name: string
  school_name?: string
  grade?: string
  class_name?: string
  class_text?: string
  school_class_id?: number
  guardian_name?: string
  guardian_phone: string
  relationship?: string
  notes?: string
}

export interface ReviewChildApplicationPayload {
  status: Exclude<ChildApplicationStatus, 'pending'>
  school_class_id?: number
  student_id?: number
  create_school_class?: boolean
  review_note?: string
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

async function applicationRequest<T>(url: string, method: 'GET' | 'POST' | 'PUT' = 'GET', data?: unknown) {
  const response = await request<ApiEnvelope<T>>({ method, url, data })
  if (response.code !== 0) {
    throw new Error(response.message || '请求失败')
  }
  return response.data
}

export function createParentChildApplication(data: CreateChildApplicationPayload) {
  return applicationRequest<ChildApplication>('/parent/child-applications', 'POST', data)
}

export function updateParentChildApplication(id: number, data: CreateChildApplicationPayload) {
  return applicationRequest<ChildApplication>(`/parent/child-applications/${id}`, 'PUT', data)
}

export function getParentChildApplications() {
  return applicationRequest<PageResult<ChildApplication>>('/parent/child-applications')
}

export function getStaffChildApplications() {
  return applicationRequest<PageResult<ChildApplication>>('/child-applications')
}

export function reviewChildApplication(id: number, data: ReviewChildApplicationPayload) {
  return applicationRequest<ChildApplication>(`/child-applications/${id}/review`, 'POST', data)
}
