import { appEnv } from '@/config/env'
import { loginParentWithWeChat, loginTeacher, logoutAuth } from '@/services/auth'
import { request } from '@/services/request'
import { syncStoreToPage, useAppStore } from '@/stores'
import { showFeedback } from '@/utils/feedback'

const appStore = useAppStore()
let stopStoreSync: (() => void) | undefined

interface ApiEnvelope<T> {
  code: number
  data: T
  message: string
}

interface MasterSummary {
  academic_terms: number
  care_classes: number
  school_classes: number
  schools: number
  students: number
}

interface SafeAreaWindowInfo {
  statusBarHeight?: number
  windowWidth?: number
}

interface SafeAreaMenuButtonInfo {
  bottom?: number
}

function resolveTopSafeStyle() {
  const fallback = '--xy-nav-safe-top: 132rpx;'

  if (typeof wx === 'undefined') {
    return fallback
  }

  try {
    const runtime = wx as unknown as {
      getMenuButtonBoundingClientRect?: () => SafeAreaMenuButtonInfo
      getSystemInfoSync?: () => SafeAreaWindowInfo
      getWindowInfo?: () => SafeAreaWindowInfo
    }
    const windowInfo = runtime.getWindowInfo?.() ?? runtime.getSystemInfoSync?.() ?? {}
    const menuButton = runtime.getMenuButtonBoundingClientRect?.() ?? {}
    const rpxRatio = 750 / (windowInfo.windowWidth || 375)
    const safeTopPx = menuButton.bottom
      ? menuButton.bottom + 14
      : (windowInfo.statusBarHeight || 24) + 52
    const safeTopRpx = Math.max(132, Math.ceil(safeTopPx * rpxRatio))

    return `--xy-nav-safe-top: ${safeTopRpx}rpx;`
  }
  catch {
    return fallback
  }
}

async function loadSummary() {
  appStore.$patch({ summaryLoading: true })
  try {
    const response = await request<ApiEnvelope<MasterSummary>>({ method: 'GET', url: '/summary' })
    if (response.code === 0) {
      appStore.$patch({ summary: response.data })
    }
  }
  catch {
    appStore.$patch({ summary: null })
  }
  finally {
    appStore.$patch({ summaryLoading: false })
  }
}

function openParentHome() {
  if (typeof wx === 'undefined') {
    return
  }
  wx.navigateTo({ url: '/pages/parent/index' })
}

function normalizePhone(value: string) {
  return value.replace(/\D/g, '')
}

Page({
  data: {
    appName: appEnv.appName,
    initialized: false,
    hasStarted: false,
    authenticated: false,
    role: 'teacher' as 'parent' | 'teacher',
    loginMode: 'choose' as 'choose' | 'teacher',
    focusedField: '' as '' | 'password' | 'phoneNumber',
    password: '',
    loginLoading: false,
    parentLoginLoading: false,
    phoneNumber: '',
    summary: null as MasterSummary | null,
    summaryLoading: false,
    topSafeStyle: '--xy-nav-safe-top: 132rpx;',
  },
  onLoad() {
    this.setData({
      topSafeStyle: resolveTopSafeStyle(),
    })
    stopStoreSync = syncStoreToPage(this, appStore, {
      select: state => ({
        initialized: state.initialized,
        hasStarted: state.hasStarted,
        authenticated: state.authenticated,
        role: state.role,
        summary: state.summary,
        summaryLoading: state.summaryLoading,
      }),
    })
    if (appStore.authenticated && appStore.role === 'teacher') {
      void loadSummary()
    }
  },
  onUnload() {
    stopStoreSync?.()
    stopStoreSync = undefined
  },
  onShow() {
    if (appStore.authenticated && appStore.role === 'teacher') {
      void loadSummary()
    }
  },
  async handleParentLogin() {
    if (this.data.parentLoginLoading) {
      return
    }
    this.setData({ parentLoginLoading: true })
    try {
      await loginParentWithWeChat()
      appStore.markAuthenticated('parent')
      openParentHome()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '微信登录失败')
    }
    finally {
      this.setData({ parentLoginLoading: false })
    }
  },
  handleOpenTeacherLogin() {
    this.setData({ loginMode: 'teacher', focusedField: '' })
  },
  handleInput(event: WechatMiniprogram.Input) {
    const field = event.currentTarget.dataset.field as 'password' | 'phoneNumber'
    const value = event.detail.value
    this.setData({ [field]: value })
  },
  handleFocus(event: WechatMiniprogram.Input) {
    const field = event.currentTarget.dataset.field as 'password' | 'phoneNumber'
    this.setData({ focusedField: field })
  },
  handleBlur() {
    this.setData({ focusedField: '' })
  },
  async handleTeacherLogin() {
    const phone = normalizePhone(this.data.phoneNumber)
    const password = this.data.password
    if (!phone || !password) {
      this.showToast('请输入手机号和密码')
      return
    }
    if (phone.length < 7) {
      this.showToast('请输入有效的手机号')
      return
    }
    this.setData({ loginLoading: true })
    try {
      const result = await loginTeacher(phone, password)
      if (result.role === 'parent') {
        this.showToast('该账号是家长账号，请从家长端登录')
        return
      }
      appStore.markAuthenticated('teacher')
      await loadSummary()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '教师登录失败')
    }
    finally {
      this.setData({ loginLoading: false })
    }
  },
  handleBackToLoginChoice() {
    this.setData({ loginMode: 'choose' })
  },
  handleStart() {
    appStore.markStarted()
    this.showToast('已进入教师工作台')
  },
  handleComingSoon(event: WechatMiniprogram.TouchEvent) {
    this.showToast(event.currentTarget.dataset.message || '该功能将在下一阶段开放')
  },
  handleOpenPickup() {
    if (typeof wx !== 'undefined') {
      wx.navigateTo({ url: '/pages/pickup/index' })
    }
  },
  handleOpenHomework() {
    if (typeof wx !== 'undefined') {
      wx.navigateTo({ url: '/pages/homework/index' })
    }
  },
  handleOpenMeals() {
    if (typeof wx !== 'undefined') {
      wx.navigateTo({ url: '/pages/meals/index' })
    }
  },
  handleOpenSummary() {
    if (typeof wx !== 'undefined') {
      wx.navigateTo({ url: '/pages/summary/index' })
    }
  },
  handleOpenApplications() {
    if (typeof wx !== 'undefined') {
      wx.navigateTo({ url: '/pages/child-applications/index' })
    }
  },
  handleOpenExceptions() {
    if (typeof wx !== 'undefined') {
      wx.navigateTo({ url: '/pages/exceptions/index' })
    }
  },
  handleOpenParent() {
    openParentHome()
  },
  handleLogout() {
    logoutAuth()
    appStore.clearAuthenticated()
    appStore.$patch({ summary: null })
    this.setData({ loginMode: 'choose', focusedField: '', phoneNumber: '', password: '' })
  },
  showToast(message: string) {
    showFeedback(this, message)
  },
})
