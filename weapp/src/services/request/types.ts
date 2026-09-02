import type { AxiosRequestConfig } from 'axios'
import type { RequestError } from './error'

export interface RequestOptions<D = unknown> extends AxiosRequestConfig<D> {
  /**
   * 是否自动携带 access token，默认 true。
   */
  withAuth?: boolean
}

export interface RequestHooks {
  getAccessToken: () => string | null
  onUnauthorized: (error: RequestError) => void | Promise<void>
  onError: (error: RequestError) => void | Promise<void>
}
