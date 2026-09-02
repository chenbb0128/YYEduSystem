import { appEnv } from '@/config/env'
import { getDevelopmentParentCode, loginParent, loginTeacher, logoutAuth } from '@/services/auth'
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

Page({
  data: {
    appName: appEnv.appName,
    initialized: false,
    hasStarted: false,
    authenticated: false,
    role: 'teacher' as 'parent' | 'teacher',
    loginMode: 'choose' as 'choose' | 'teacher',
    focusedField: '' as '' | 'username' | 'password',
    username: '',
    password: '',
    loginLoading: false,
    summary: null as MasterSummary | null,
    summaryLoading: false,
  },
  onLoad() {
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
  handleParentLogin() {
    this.setData({ loginLoading: true })
    const login = (code: string) => loginParent(code)
      .then(() => {
        appStore.markAuthenticated('parent')
        if (typeof wx !== 'undefined') {
          wx.navigateTo({ url: '/pages/parent/index' })
        }
      })
      .catch(error => this.showToast(error instanceof Error ? error.message : '家长登录失败'))
      .finally(() => this.setData({ loginLoading: false }))

    if (typeof wx === 'undefined') {
      void login(getDevelopmentParentCode())
      return
    }
    wx.login({
      success: (result) => {
        if (!result.code) {
          this.setData({ loginLoading: false })
          this.showToast('微信登录未获取到授权凭证')
          return
        }
        void login(result.code)
      },
      fail: (error) => {
        this.setData({ loginLoading: false })
        this.showToast(error.errMsg || '微信登录失败')
      },
    })
  },
  handleOpenTeacherLogin() {
    this.setData({ loginMode: 'teacher' })
  },
  handleInput(event: WechatMiniprogram.Input) {
    const field = event.currentTarget.dataset.field as 'username' | 'password'
    this.setData({ [field]: event.detail.value })
  },
  handleFocus(event: WechatMiniprogram.Input) {
    const field = event.currentTarget.dataset.field as 'username' | 'password'
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
  handleOpenParent() {
    if (typeof wx !== 'undefined') {
      wx.navigateTo({ url: '/pages/parent/index' })
    }
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
