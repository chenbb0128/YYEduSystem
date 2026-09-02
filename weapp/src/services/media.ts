import { appEnv } from '@/config/env'

/**
 * 将业务接口返回的媒体路径转换成可访问地址。
 * 接送、餐食和作业照片可能是后端生成的短时签名 URL，也可能是相对路径。
 */
export function mediaURL(path?: string) {
  if (!path) {
    return ''
  }
  if (/^https?:\/\//i.test(path)) {
    return path
  }

  const baseURL = appEnv.apiBaseUrl
    .replace(/\/api\/v1\/?$/, '')
    .replace(/\/+$/, '')
  return `${baseURL}${path.startsWith('/') ? path : `/${path}`}`
}
