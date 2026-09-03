import { request } from '@/services/request'

export interface SchoolClassRecord {
  id: number
  school_id: number
  term_id: number
  grade: string
  name: string
  status: string
}

export interface SchoolRecord {
  id: number
  name: string
  address: string
  contact_phone: string
  status: string
}

export interface StudentRecord {
  id: number
  school_id: number
  term_id: number
  school_class_id: number
  name: string
  status: 'active' | 'inactive'
}

interface ApiEnvelope<T> {
  code: number
  data: T
  message: string
}

export interface PageResult<T> {
  items: T[]
  total: number
}

async function masterDataRequest<T>(url: string) {
  const response = await request<ApiEnvelope<T>>({ method: 'GET', url })
  if (response.code !== 0) {
    throw new Error(response.message || '基础资料加载失败')
  }
  return response.data
}

export function getSchoolClasses() {
  return masterDataRequest<PageResult<SchoolClassRecord>>('/school-classes')
}

export function getSchools() {
  return masterDataRequest<PageResult<SchoolRecord>>('/schools')
}

export function getStudents() {
  return masterDataRequest<PageResult<StudentRecord>>('/students')
}
