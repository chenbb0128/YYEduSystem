import { request } from '@/services/request'

export interface ParentPrivacyConsent {
  accepted: boolean
  policy_version: string
  current_policy_version: string
  consented_at?: string
}

interface ApiEnvelope<T> {
  code: number
  data: T
  message: string
}

async function privacyRequest<T>(url: string, method: 'GET' | 'POST' = 'GET', data?: unknown) {
  const response = await request<ApiEnvelope<T>>({ method, url, data })
  if (response.code !== 0) {
    throw new Error(response.message || '隐私设置请求失败')
  }
  return response.data
}

export function getParentPrivacyConsent() {
  return privacyRequest<ParentPrivacyConsent>('/parent/privacy-consent')
}

export function recordParentPrivacyConsent(policyVersion: string) {
  return privacyRequest<ParentPrivacyConsent>('/parent/privacy-consent', 'POST', { policy_version: policyVersion })
}
