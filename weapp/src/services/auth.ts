import { request } from '@/services/request'
import { clearAuthStorage, getPrincipal, getRefreshToken, getStorage, setAccessToken, setPrincipal, setRefreshToken, setStorage } from '@/utils/storage'

export type AuthPrincipal = 'parent' | 'user'
export type PhoneLoginRoleKey = 'parent' | 'staff'

export interface AuthTokenResult {
  accessToken: string
  refreshToken: string
  expiresIn: number
  principal: AuthPrincipal
  role: string
}

export interface PhoneLoginRole {
  key: PhoneLoginRoleKey
  principal: AuthPrincipal
  role: string
  label: string
  available: boolean
  message?: string
  accessToken?: string
  refreshToken?: string
  expiresIn?: number
}

export interface PhoneLoginResult {
  phone: string
  masked_phone: string
  roles: PhoneLoginRole[]
}

export interface PhoneCodeResult {
  phone: string
  masked_phone: string
  expires_in: number
  retry_after: number
  debug_code?: string
}

interface ApiEnvelope<T> {
  code: number
  data: T
  message: string
}

const phoneLoginPhoneKey = 'auth.phone-login-phone'
const phoneLoginMaskedPhoneKey = 'auth.phone-login-masked-phone'

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

export function loginByPhone(phone: string, code: string) {
  return authRequest<PhoneLoginResult>('/auth/phone-login', { phone, code }).then((result) => {
    setStorage(phoneLoginPhoneKey, result.phone)
    setStorage(phoneLoginMaskedPhoneKey, result.masked_phone)
    return result
  })
}

export function requestPhoneCode(phone: string) {
  return authRequest<PhoneCodeResult>('/auth/phone-code', { phone })
}

export function savePhoneLoginRole(role: PhoneLoginRole) {
  if (!role.available || !role.accessToken || !role.refreshToken) {
    throw new Error(role.message || '该入口暂不可登录')
  }
  const result: AuthTokenResult = {
    accessToken: role.accessToken,
    refreshToken: role.refreshToken,
    expiresIn: role.expiresIn || 0,
    principal: role.principal,
    role: role.role,
  }
  return saveLogin(result)
}

export function loginParent(code: string, nickname = '', avatar = '', openid?: string) {
  return authRequest<AuthTokenResult>('/auth/parent/wechat', { code, nickname, avatar, ...(openid ? { openid } : {}) }).then(saveLogin)
}

/** 正式环境使用 wx.login 返回的临时 code，由服务端 code2Session 换取 OpenID。 */
export function loginParentWithWeChat() {
  if (typeof wx === 'undefined') {
    return Promise.reject(new Error('当前环境不支持微信登录'))
  }
  return new Promise<AuthTokenResult>((resolve, reject) => {
    wx.login({
      success: (result) => {
        if (!result.code) {
          reject(new Error('微信登录没有返回有效凭证'))
          return
        }
        loginParent(result.code).then(resolve).catch(reject)
      },
      fail: error => reject(new Error(error.errMsg || '微信登录失败')),
    })
  })
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

export function getStoredPhoneLoginPhone() {
  return getStorage<string>(phoneLoginPhoneKey, '') || ''
}

export function getStoredPhoneLoginMaskedPhone() {
  return getStorage<string>(phoneLoginMaskedPhoneKey, '') || ''
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
