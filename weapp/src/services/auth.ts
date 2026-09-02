import { request } from '@/services/request'
import { clearAuthStorage, getPrincipal, getRefreshToken, getStorage, setAccessToken, setPrincipal, setRefreshToken, setStorage } from '@/utils/storage'

export type AuthPrincipal = 'parent' | 'user'

export interface AuthTokenResult {
  accessToken: string
  refreshToken: string
  expiresIn: number
  principal: AuthPrincipal
  role: string
}

interface ApiEnvelope<T> {
  code: number
  data: T
  message: string
}

async function authRequest<T>(url: string, data: unknown) {
  const response = await request<ApiEnvelope<T>>({ method: 'POST', url, data, withAuth: false })
  if (response.code !== 0) {
    throw new Error(response.message || '登录失败')
  }
  return response.data
}

async function saveLogin(result: AuthTokenResult) {
  setAccessToken(result.accessToken)
  setRefreshToken(result.refreshToken)
  setPrincipal(result.principal)
  return result
}

export function loginTeacher(username: string, password: string) {
  return authRequest<AuthTokenResult>('/auth/login', { username, password }).then(saveLogin)
}

export function loginParent(code: string, nickname = '', avatar = '', openid?: string) {
  return authRequest<AuthTokenResult>('/auth/parent/wechat', { code, nickname, avatar, ...(openid ? { openid } : {}) }).then(saveLogin)
}

export async function refreshAuth() {
  const refreshToken = getRefreshToken()
  if (!refreshToken) {
    return null
  }
  try {
    const result = await authRequest<AuthTokenResult>('/auth/refresh', { refreshToken })
    return saveLogin(result)
  }
  catch {
    clearAuthStorage()
    return null
  }
}

export function logoutAuth() {
  clearAuthStorage()
}

export function getStoredPrincipal() {
  return getPrincipal()
}

export function getDevelopmentParentCode() {
  const key = 'auth.local-parent-code'
  const existing = getStorage<string>(key)
  if (existing) {
    return existing
  }
  const generated = `local-parent-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
  setStorage(key, generated)
  return generated
}
