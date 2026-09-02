import axios from 'axios'

export type RequestErrorCode
  = | 'CANCELED'
    | 'HTTP'
    | 'NETWORK'
    | 'TIMEOUT'
    | 'UNAUTHORIZED'
    | 'UNKNOWN'

interface RequestErrorOptions {
  code: RequestErrorCode
  status?: number
  data?: unknown
  method?: string
  url?: string
  originalError?: unknown
}

export class RequestError extends Error {
  readonly code: RequestErrorCode
  readonly status?: number
  readonly data?: unknown
  readonly method?: string
  readonly url?: string
  readonly originalError?: unknown

  constructor(message: string, options: RequestErrorOptions) {
    super(message)
    this.name = 'RequestError'
    this.code = options.code
    this.status = options.status
    this.data = options.data
    this.method = options.method
    this.url = options.url
    this.originalError = options.originalError
  }
}

function getResponseMessage(data: unknown) {
  if (!data || typeof data !== 'object') {
    return undefined
  }

  const message = Reflect.get(data, 'message')
  return typeof message === 'string' && message.trim() ? message : undefined
}

export function normalizeRequestError(error: unknown) {
  if (error instanceof RequestError) {
    return error
  }

  if (!axios.isAxiosError(error)) {
    return new RequestError(
      error instanceof Error ? error.message : '请求失败',
      {
        code: 'UNKNOWN',
        originalError: error,
      },
    )
  }

  const status = error.response?.status
  const data = error.response?.data
  const common = {
    status,
    data,
    method: error.config?.method?.toUpperCase(),
    url: error.config?.url,
    originalError: error,
  }

  if (error.code === 'ERR_CANCELED') {
    return new RequestError('请求已取消', { ...common, code: 'CANCELED' })
  }

  if (error.code === 'ECONNABORTED' || error.code === 'ETIMEDOUT') {
    return new RequestError('请求超时，请稍后重试', { ...common, code: 'TIMEOUT' })
  }

  if (status === 401) {
    return new RequestError(
      getResponseMessage(data) ?? '登录状态已失效',
      { ...common, code: 'UNAUTHORIZED' },
    )
  }

  if (typeof status === 'number') {
    return new RequestError(
      getResponseMessage(data) ?? `请求失败（${status}）`,
      { ...common, code: 'HTTP' },
    )
  }

  if (error.code === 'ERR_NETWORK') {
    return new RequestError('网络连接失败，请检查网络', { ...common, code: 'NETWORK' })
  }

  return new RequestError(error.message || '请求失败', { ...common, code: 'UNKNOWN' })
}

export function isRequestError(error: unknown): error is RequestError {
  return error instanceof RequestError
}
