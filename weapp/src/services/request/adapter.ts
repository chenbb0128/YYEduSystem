import type { AxiosAdapter, AxiosResponse, InternalAxiosRequestConfig } from 'axios'
import { AxiosError, AxiosHeaders } from 'axios'

export function buildUrl(baseURL: string | undefined, url: string | undefined) {
  if (!url) {
    return baseURL ?? ''
  }

  if (/^https?:\/\//i.test(url)) {
    return url
  }

  const base = (baseURL ?? '').replace(/\/+$/, '')
  const path = url.replace(/^\/+/, '')
  return base ? `${base}/${path}` : path
}

function serializeParamValue(value: unknown) {
  if (value instanceof Date) {
    return value.toISOString()
  }

  if (value && typeof value === 'object') {
    try {
      return JSON.stringify(value)
    }
    catch {
      return String(value)
    }
  }

  return String(value)
}

export function appendParams(url: string, params: unknown) {
  if (!params || typeof params !== 'object') {
    return url
  }

  const entries = Object.entries(params as Record<string, unknown>).flatMap(([key, value]) => {
    if (value === null || typeof value === 'undefined') {
      return []
    }

    const values = Array.isArray(value) ? value : [value]
    return values.map((item) => {
      const encodedKey = encodeURIComponent(key)
      const encodedValue = encodeURIComponent(serializeParamValue(item))
      return `${encodedKey}=${encodedValue}`
    })
  })

  if (!entries.length) {
    return url
  }

  const hashIndex = url.indexOf('#')
  const hash = hashIndex >= 0 ? url.slice(hashIndex) : ''
  const urlWithoutHash = hashIndex >= 0 ? url.slice(0, hashIndex) : url
  const separator = urlWithoutHash.includes('?') ? '&' : '?'

  return `${urlWithoutHash}${separator}${entries.join('&')}${hash}`
}

function getHeaders(config: InternalAxiosRequestConfig) {
  if (!config.headers) {
    return undefined
  }

  return AxiosHeaders.from(config.headers).toJSON() as WechatMiniprogram.IAnyObject
}

export const miniProgramAdapter: AxiosAdapter = config => new Promise((resolve, reject) => {
  if (typeof wx === 'undefined') {
    reject(new AxiosError('当前运行环境不支持 wx.request', AxiosError.ERR_NOT_SUPPORT, config))
    return
  }

  const url = appendParams(buildUrl(config.baseURL, config.url), config.params)
  if (!url || !/^https?:\/\//i.test(url)) {
    reject(new AxiosError('请先配置有效的 VITE_API_BASE_URL', AxiosError.ERR_BAD_REQUEST, config))
    return
  }

  const signal = config.signal
  if (signal?.aborted) {
    reject(new AxiosError('canceled', AxiosError.ERR_CANCELED, config))
    return
  }

  let settled = false
  let abortHandler: (() => void) | undefined
  let requestTask: WechatMiniprogram.RequestTask

  const cleanup = () => {
    if (abortHandler && typeof signal?.removeEventListener === 'function') {
      signal.removeEventListener('abort', abortHandler)
    }
  }

  const resolveOnce = (response: AxiosResponse) => {
    if (settled) {
      return
    }

    settled = true
    cleanup()
    resolve(response)
  }

  const rejectOnce = (error: AxiosError) => {
    if (settled) {
      return
    }

    settled = true
    cleanup()
    reject(error)
  }

  try {
    requestTask = wx.request({
      url,
      method: (config.method?.toUpperCase() ?? 'GET') as WechatMiniprogram.RequestOption['method'],
      data: config.data,
      header: getHeaders(config),
      timeout: config.timeout,
      responseType: config.responseType === 'arraybuffer' ? 'arraybuffer' : 'text',
      success(result) {
        const response: AxiosResponse = {
          data: result.data,
          status: result.statusCode,
          statusText: '',
          headers: result.header ?? {},
          config,
          request: requestTask,
        }
        const validateStatus = config.validateStatus

        if (!validateStatus || validateStatus(result.statusCode)) {
          resolveOnce(response)
          return
        }

        rejectOnce(new AxiosError(
          `Request failed with status code ${result.statusCode}`,
          AxiosError.ERR_BAD_RESPONSE,
          config,
          requestTask,
          response,
        ))
      },
      fail(error) {
        const timeout = /timeout/i.test(error.errMsg)
        rejectOnce(new AxiosError(
          error.errMsg || 'Network Error',
          timeout ? AxiosError.ETIMEDOUT : AxiosError.ERR_NETWORK,
          config,
          requestTask,
        ))
      },
    })
  }
  catch (error) {
    rejectOnce(new AxiosError(
      error instanceof Error ? error.message : 'wx.request failed',
      AxiosError.ERR_NETWORK,
      config,
    ))
    return
  }

  if (!signal || typeof signal.addEventListener !== 'function') {
    return
  }

  abortHandler = () => {
    requestTask.abort()
    rejectOnce(new AxiosError('canceled', AxiosError.ERR_CANCELED, config, requestTask))
  }

  signal.addEventListener('abort', abortHandler, { once: true })

  if (signal.aborted) {
    abortHandler()
  }
})
