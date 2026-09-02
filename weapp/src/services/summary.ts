import { request } from '@/services/request'

export interface DailySummary {
  id: number
  summary_date: string
  content: string
  child_updates?: Record<string, string>
  status: 'draft' | 'published' | 'closed' | 'withdrawn'
  version: number
  created_by_name: string
  generated_at?: string
  published_at?: string
  closed_at?: string
  withdrawn_at?: string
  withdrawal_reason?: string
  correction_reason?: string
  read_at?: string
  created_at: string
  updated_at: string
}

export interface DailySummaryVersion {
  id: number
  version: number
  action: string
  content: string
  child_updates?: Record<string, string>
  reason?: string
  created_by_name: string
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

async function summaryRequest<T>(
  url: string,
  method: 'GET' | 'POST' | 'PUT' = 'GET',
  data?: unknown,
  params?: Record<string, string>,
) {
  const response = await request<ApiEnvelope<T>>({ method, url, data, params })
  if (response.code !== 0) {
    throw new Error(response.message || '总结请求失败')
  }
  return response.data
}

export function getDailySummaries(date?: string) {
  return summaryRequest<PageResult<DailySummary>>(
    '/daily-summaries',
    'GET',
    undefined,
    date ? { date } : undefined,
  )
}

export function generateDailySummary(summaryDate: string) {
  return summaryRequest<DailySummary>('/daily-summaries/generate', 'POST', {
    summary_date: summaryDate,
  })
}

export function updateDailySummary(
  id: number,
  data: { content: string, child_updates?: Record<string, string> },
) {
  return summaryRequest<DailySummary>(`/daily-summaries/${id}`, 'PUT', data)
}

export function getDailySummaryVersions(id: number) {
  return summaryRequest<{ items: DailySummaryVersion[], total: number }>(`/daily-summaries/${id}/versions`)
}

export function publishDailySummary(id: number) {
  return summaryRequest<DailySummary>(`/daily-summaries/${id}/publish`, 'POST', {})
}

export function closeDailySummary(id: number) {
  return summaryRequest<DailySummary>(`/daily-summaries/${id}/close`, 'POST', {})
}

export function withdrawDailySummary(id: number, reason: string) {
  return summaryRequest<DailySummary>(`/daily-summaries/${id}/withdraw`, 'POST', { reason })
}

export function correctDailySummary(id: number, data: { content: string, child_updates?: Record<string, string>, reason: string }) {
  return summaryRequest<DailySummary>(`/daily-summaries/${id}/correct`, 'POST', data)
}

export function getParentDailySummary(date: string) {
  return summaryRequest<DailySummary | null>('/parent/daily-summary', 'GET', undefined, { date })
}

export function markParentDailySummaryRead(id: number) {
  return summaryRequest<{ id: number, read: boolean, read_at?: string }>(`/parent/daily-summary/${id}/read`, 'POST', {})
}
