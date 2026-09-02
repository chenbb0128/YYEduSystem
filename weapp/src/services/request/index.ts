import type { AxiosResponse } from 'axios'
import type { RequestError } from './error'
import type { RequestHooks, RequestOptions } from './types'
import axios, { AxiosHeaders } from 'axios'
import { appEnv } from '@/config/env'
import { reportError } from '@/services/monitoring'
import { clearAuthStorage, getAccessToken } from '@/utils/storage'
import { miniProgramAdapter } from './adapter'
import { isRequestError, normalizeRequestError } from './error'

const useMiniProgramAdapter = typeof wx !== 'undefined'

export const http = axios.create({
  baseURL: appEnv.apiBaseUrl,
  timeout: appEnv.requestTimeout,
  adapter: useMiniProgramAdapter ? miniProgramAdapter : undefined,
})

const requestHooks: RequestHooks = {
  getAccessToken,
  onUnauthorized() {
    clearAuthStorage()
  },
  onError(error) {
    reportError(error, {
      source: 'request',
      metadata: {
        code: error.code,
        method: error.method,
        status: error.status,
        url: error.url,
      },
    })
  },
}

function invokeHook(hook: (error: RequestError) => void | Promise<void>, error: RequestError) {
  try {
    const result = hook(error)
    if (result instanceof Promise) {
      void result.catch(hookError => reportError(hookError, { source: 'request-hook' }))
    }
  }
  catch (hookError) {
    reportError(hookError, { source: 'request-hook' })
  }
}

http.interceptors.response.use(
  response => response,
  (error) => {
    const requestError = normalizeRequestError(error)

    if (requestError.code === 'UNAUTHORIZED') {
      invokeHook(requestHooks.onUnauthorized, requestError)
    }

    if (requestError.code !== 'CANCELED') {
      invokeHook(requestHooks.onError, requestError)
    }

    return Promise.reject(requestError)
  },
)

function prepareRequestConfig<D>(config: RequestOptions<D>) {
  const { withAuth = true, ...axiosConfig } = config
  if (!withAuth) {
    return axiosConfig
  }

  const token = requestHooks.getAccessToken()
  if (!token) {
    return axiosConfig
  }

  const headers = AxiosHeaders.from(axiosConfig.headers as AxiosHeaders | undefined)
  headers.set('Authorization', `Bearer ${token}`)

  return {
    ...axiosConfig,
    headers,
  }
}

/**
 * 替换请求层与业务系统的连接点，例如登录失效跳转和错误上报。
 * 返回值可用于恢复替换前的 hooks，便于测试或热更新。
 */
export function configureRequestHooks(hooks: Partial<RequestHooks>) {
  const previousHooks = { ...requestHooks }
  Object.assign(requestHooks, hooks)

  return () => {
    Object.assign(requestHooks, previousHooks)
  }
}

export function requestRaw<T = unknown, D = unknown>(config: RequestOptions<D>) {
  return http.request<T, AxiosResponse<T>, D>(prepareRequestConfig(config))
}

export async function request<T = unknown, D = unknown>(config: RequestOptions<D>) {
  const method = (config.method || 'GET').toUpperCase()
  const canRetry = method === 'GET' || method === 'HEAD'
  const maxAttempts = canRetry ? 2 : 1
  let attempt = 0

  while (attempt < maxAttempts) {
    try {
      const response = await requestRaw<T, D>(config)
      return response.data
    }
    catch (error) {
      attempt += 1
      if (!canRetry || attempt >= maxAttempts || !isRequestError(error) || (error.code !== 'NETWORK' && error.code !== 'TIMEOUT')) {
        throw error
      }
      await new Promise(resolve => setTimeout(resolve, 250))
    }
  }

  throw new Error('请求失败')
}

export { isRequestError, RequestError } from './error'
export type { RequestHooks, RequestOptions } from './types'
