import { appEnv } from '@/config/env'
import { request } from '@/services/request'

export type MessageSubscriptionKind = 'homework' | 'leave' | 'meal' | 'pickup' | 'summary'
export type MessageSubscriptionStatus = 'accept' | 'ban' | 'filter' | 'reject' | 'unknown'

export interface MessageSubscription {
  kind: MessageSubscriptionKind
  status: MessageSubscriptionStatus
  template_version?: string
  authorized_at?: string
  updated_at: string
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

const templates = ([
  { kind: 'pickup', templateID: appEnv.subscribeTemplates.pickup },
  { kind: 'meal', templateID: appEnv.subscribeTemplates.meal },
  { kind: 'homework', templateID: appEnv.subscribeTemplates.homework },
  { kind: 'leave', templateID: appEnv.subscribeTemplates.leave },
  { kind: 'summary', templateID: appEnv.subscribeTemplates.summary },
] as Array<{ kind: MessageSubscriptionKind, templateID: string }>).filter(item => item.templateID)

async function subscriptionRequest<T>(
  url: string,
  method: 'GET' | 'POST' = 'GET',
  data?: unknown,
) {
  const response = await request<ApiEnvelope<T>>({ method, url, data })
  if (response.code !== 0) {
    throw new Error(response.message || '消息授权状态保存失败')
  }
  return response.data
}

export function getParentSubscriptions() {
  return subscriptionRequest<PageResult<MessageSubscription>>('/parent/subscriptions')
}

export function requestParentSubscriptions() {
  if (typeof wx === 'undefined') {
    return Promise.resolve([] as MessageSubscription[])
  }
  if (!templates.length) {
    return Promise.reject(new Error('管理员尚未配置微信订阅消息模板'))
  }
  return new Promise<MessageSubscription[]>((resolve, reject) => {
    const results: Record<string, string> = {}
    const chunks: Array<typeof templates> = []
    for (let index = 0; index < templates.length; index += 3) {
      chunks.push(templates.slice(index, index + 3))
    }
    const requestChunk = (chunk: Array<typeof templates[number]>) => new Promise<void>((chunkResolve) => {
      wx.requestSubscribeMessage({
        tmplIds: chunk.map(item => item.templateID),
        success: (result) => {
          for (const item of chunk) {
            results[item.templateID] = result[item.templateID] || 'unknown'
          }
          chunkResolve()
        },
        // 授权弹窗被关闭、模板失效或微信环境异常时，保存 unknown 状态。
        // 这不会阻断站内通知和其他业务流程，家长稍后仍可再次点击开启。
        fail: () => {
          for (const item of chunk) {
            results[item.templateID] = 'unknown'
          }
          chunkResolve()
        },
      })
    })
    const requestAll = async () => {
      for (const chunk of chunks) {
        await requestChunk(chunk)
      }
      const subscriptions = templates.map(item => ({
        kind: item.kind,
        status: (results[item.templateID] || 'unknown') as MessageSubscriptionStatus,
        template_version: item.templateID,
      }))
      const saved = await subscriptionRequest<PageResult<MessageSubscription>>('/parent/subscriptions', 'POST', { subscriptions })
      resolve(saved.items)
    }
    void requestAll().catch(reject)
  })
}

export function hasConfiguredSubscriptionTemplates() {
  return templates.length > 0
}
