import { appEnv } from '@/config/env'
import { reportError } from '@/services/monitoring'

interface WebStorage {
  getItem: (key: string) => string | null
  setItem: (key: string, value: string) => void
  removeItem: (key: string) => void
}

const storageKeys = {
  accessToken: 'auth.access-token',
  refreshToken: 'auth.refresh-token',
  principal: 'auth.principal',
} as const

function getWebStorage(): WebStorage | undefined {
  if (typeof localStorage === 'undefined') {
    return undefined
  }

  return localStorage as unknown as WebStorage
}

export function resolveStorageKey(key: string) {
  const normalizedKey = key.trim()
  if (!normalizedKey) {
    throw new Error('Storage key cannot be empty')
  }

  return `${appEnv.storagePrefix}:${normalizedKey}`
}

export function getStorage<T>(key: string, fallback: T | null = null): T | null {
  const resolvedKey = resolveStorageKey(key)

  try {
    if (typeof wx !== 'undefined') {
      const value = wx.getStorageSync(resolvedKey)
      return value === '' || typeof value === 'undefined' ? fallback : value as T
    }

    const value = getWebStorage()?.getItem(resolvedKey)
    return value === null || typeof value === 'undefined' ? fallback : JSON.parse(value) as T
  }
  catch (error) {
    reportError(error, {
      source: 'storage',
      metadata: { operation: 'get', key: resolvedKey },
    })
    return fallback
  }
}

export function setStorage<T>(key: string, value: T) {
  const resolvedKey = resolveStorageKey(key)

  try {
    if (typeof wx !== 'undefined') {
      wx.setStorageSync(resolvedKey, value)
      return true
    }

    getWebStorage()?.setItem(resolvedKey, JSON.stringify(value))
    return true
  }
  catch (error) {
    reportError(error, {
      source: 'storage',
      metadata: { operation: 'set', key: resolvedKey },
    })
    return false
  }
}

export function removeStorage(key: string) {
  const resolvedKey = resolveStorageKey(key)

  try {
    if (typeof wx !== 'undefined') {
      wx.removeStorageSync(resolvedKey)
      return true
    }

    getWebStorage()?.removeItem(resolvedKey)
    return true
  }
  catch (error) {
    reportError(error, {
      source: 'storage',
      metadata: { operation: 'remove', key: resolvedKey },
    })
    return false
  }
}

export function getAccessToken() {
  return getStorage<string>(storageKeys.accessToken)
}

export function setAccessToken(token: string) {
  return setStorage(storageKeys.accessToken, token)
}

export function clearAccessToken() {
  return removeStorage(storageKeys.accessToken)
}

export function getRefreshToken() {
  return getStorage<string>(storageKeys.refreshToken)
}

export function setRefreshToken(token: string) {
  return setStorage(storageKeys.refreshToken, token)
}

export function clearRefreshToken() {
  return removeStorage(storageKeys.refreshToken)
}

export function getPrincipal() {
  return getStorage<'parent' | 'user'>(storageKeys.principal)
}

export function setPrincipal(principal: 'parent' | 'user') {
  return setStorage(storageKeys.principal, principal)
}

export function clearAuthStorage() {
  clearAccessToken()
  clearRefreshToken()
  removeStorage(storageKeys.principal)
}

export { storageKeys }
