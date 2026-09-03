const rawTimeout = Number(import.meta.env.VITE_API_TIMEOUT ?? 10000)
const storagePrefix = import.meta.env.VITE_STORAGE_PREFIX?.trim() || 'weapp-template'

export const appEnv = Object.freeze({
  appName: import.meta.env.VITE_APP_NAME?.trim() || '豆芽成长助手',
  apiBaseUrl: import.meta.env.VITE_API_BASE_URL?.trim() || '',
  requestTimeout: Number.isFinite(rawTimeout) && rawTimeout > 0 ? rawTimeout : 10000,
  storagePrefix,
  enableLog: import.meta.env.VITE_ENABLE_LOG !== 'false',
  subscribeTemplates: Object.freeze({
    pickup: import.meta.env.VITE_WECHAT_SUBSCRIBE_PICKUP_TEMPLATE_ID?.trim() || '',
    meal: import.meta.env.VITE_WECHAT_SUBSCRIBE_MEAL_TEMPLATE_ID?.trim() || '',
    homework: import.meta.env.VITE_WECHAT_SUBSCRIBE_HOMEWORK_TEMPLATE_ID?.trim() || '',
    leave: import.meta.env.VITE_WECHAT_SUBSCRIBE_LEAVE_TEMPLATE_ID?.trim() || '',
    summary: import.meta.env.VITE_WECHAT_SUBSCRIBE_SUMMARY_TEMPLATE_ID?.trim() || '',
  }),
})
