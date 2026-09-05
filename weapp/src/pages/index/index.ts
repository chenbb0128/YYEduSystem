import type { PhoneLoginRole } from '@/services/auth'
import { appEnv } from '@/config/env'
import { getStoredPhoneLoginPhone, loginByPhone, loginParentWithWeChat, loginTeacher, logoutAuth, requestPhoneCode, savePhoneLoginRole } from '@/services/auth'
import { request } from '@/services/request'
import { syncStoreToPage, useAppStore } from '@/stores'
import { showFeedback } from '@/utils/feedback'

const appStore = useAppStore()
let stopStoreSync: (() => void) | undefined
let phoneCodeTimer: ReturnType<typeof setInterval> | undefined

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

function findPhoneRole(roles: PhoneLoginRole[], key: PhoneLoginRole['key']) {
  return roles.find(item => item.key === key)
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
    focusedField: '' as '' | 'password' | 'phoneCode' | 'phoneNumber' | 'username',
    username: '',
    password: '',
    loginLoading: false,
    parentLoginLoading: false,
    phoneNumber: '',
    phoneCode: '',
    phoneCodeSending: false,
    phoneCodeCountdown: 0,
    phoneCodeTarget: '',
    phoneAuthorized: false,
    phoneAuthLoading: false,
    phoneAuthLabel: '',
    phoneLoginRoles: [] as PhoneLoginRole[],
    staffRoleAvailable: false,
    staffRoleMessage: '手机号登录后自动识别教职工资格',
    summary: null as MasterSummary | null,
    summaryLoading: false,
    topSafeStyle: '--xy-nav-safe-top: 132rpx;',
  },
  onLoad() {
    this.setData({
      topSafeStyle: resolveTopSafeStyle(),
      phoneNumber: getStoredPhoneLoginPhone(),
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
    if (phoneCodeTimer) {
      clearInterval(phoneCodeTimer)
      phoneCodeTimer = undefined
    }
  },
  onShow() {
    if (appStore.authenticated && appStore.role === 'teacher') {
      void loadSummary()
    }
  },
  completePhoneAuthorization(label: string, roles: PhoneLoginRole[]) {
    const staffRole = findPhoneRole(roles, 'staff')
    this.setData({
      phoneAuthorized: true,
      phoneAuthLoading: false,
      phoneAuthLabel: label,
      phoneLoginRoles: roles,
      staffRoleAvailable: Boolean(staffRole?.available),
      staffRoleMessage: staffRole?.message || '该手机号未登记为教职工，请联系管理员开通',
      loginMode: 'choose',
    })
    this.showToast('手机号验证成功，请选择身份')
  },
  startPhoneCodeCountdown(seconds: number) {
    if (phoneCodeTimer) {
      clearInterval(phoneCodeTimer)
    }
    let remaining = Math.max(1, Math.ceil(seconds))
    this.setData({ phoneCodeCountdown: remaining })
    phoneCodeTimer = setInterval(() => {
      remaining -= 1
      if (remaining <= 0) {
        if (phoneCodeTimer) {
          clearInterval(phoneCodeTimer)
          phoneCodeTimer = undefined
        }
        this.setData({ phoneCodeCountdown: 0 })
        return
      }
      this.setData({ phoneCodeCountdown: remaining })
    }, 1000)
  },
  async handleGetPhoneCode() {
    if (this.data.phoneCodeSending || this.data.phoneCodeCountdown > 0) {
      return
    }
    const phone = normalizePhone(this.data.phoneNumber)
    if (!phone) {
      this.showToast('请先输入手机号')
      return
    }
    this.setData({ phoneCodeSending: true })
    try {
      const result = await requestPhoneCode(phone)
      if (normalizePhone(this.data.phoneNumber) !== phone) {
        return
      }
      this.setData({
        phoneNumber: result.phone || phone,
        phoneCode: result.debug_code || '',
        phoneCodeTarget: result.phone || phone,
      })
      this.startPhoneCodeCountdown(result.retry_after || 60)
      this.showToast(result.debug_code ? '本地测试验证码已生成' : '验证码已发送，请注意查收')
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '验证码发送失败')
    }
    finally {
      this.setData({ phoneCodeSending: false })
    }
  },
  async handlePhoneLogin() {
    if (this.data.phoneAuthLoading) {
      return
    }
    const phone = normalizePhone(this.data.phoneNumber)
    const code = this.data.phoneCode.trim()
    if (!phone || !code) {
      this.showToast('请输入手机号和验证码')
      return
    }
    this.setData({ phoneAuthLoading: true })
    try {
      const result = await loginByPhone(phone, code)
      this.completePhoneAuthorization(result.masked_phone || result.phone, result.roles)
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '手机号登录失败')
    }
    finally {
      this.setData({ phoneAuthLoading: false })
    }
  },
  async handleParentLogin() {
    if (this.data.parentLoginLoading) {
      return
    }
    // Phone login is retained for local/staff compatibility. In a real
    // mini-program, parent entry uses the official wx.login flow directly.
    if (this.data.phoneAuthorized) {
      const parentRole = findPhoneRole(this.data.phoneLoginRoles, 'parent')
      if (!parentRole) {
        this.showToast('当前手机号暂不能进入家长端')
        return
      }
      try {
        savePhoneLoginRole(parentRole)
        appStore.markAuthenticated('parent')
        openParentHome()
      }
      catch (error) {
        this.showToast(error instanceof Error ? error.message : '家长登录失败')
      }
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
    if (!this.data.phoneAuthorized) {
      this.showToast('请先完成手机号验证码登录')
      return
    }
    const staffRole = findPhoneRole(this.data.phoneLoginRoles, 'staff')
    if (!staffRole?.available) {
      this.showToast(staffRole?.message || '该手机号未登记为教职工，请联系管理员开通')
      return
    }
    try {
      savePhoneLoginRole(staffRole)
      appStore.markAuthenticated('teacher')
      void loadSummary()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '老师登录失败')
    }
  },
  handleInput(event: WechatMiniprogram.Input) {
    const field = event.currentTarget.dataset.field as 'password' | 'phoneCode' | 'phoneNumber' | 'username'
    const value = event.detail.value
    if (field === 'phoneNumber' && this.data.phoneCodeTarget && normalizePhone(value) !== this.data.phoneCodeTarget) {
      if (phoneCodeTimer) {
        clearInterval(phoneCodeTimer)
        phoneCodeTimer = undefined
      }
      this.setData({ phoneNumber: value, phoneCode: '', phoneCodeCountdown: 0, phoneCodeTarget: '' })
      return
    }
    this.setData({ [field]: value })
  },
  handleFocus(event: WechatMiniprogram.Input) {
    const field = event.currentTarget.dataset.field as 'password' | 'phoneCode' | 'phoneNumber' | 'username'
    this.setData({ focusedField: field })
  },
  handleBlur() {
    this.setData({ focusedField: '' })
  },
  async handleTeacherLogin() {
    const username = this.data.username.trim()
    const password = this.data.password
    if (!username || !password) {
      this.showToast('请输入教师账号和密码')
      return
    }
    this.setData({ loginLoading: true })
    try {
      const result = await loginTeacher(username, password)
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
    this.setData({ loginMode: 'choose', focusedField: '', username: '', password: '' })
  },
  showToast(message: string) {
    showFeedback(this, message)
  },
})
