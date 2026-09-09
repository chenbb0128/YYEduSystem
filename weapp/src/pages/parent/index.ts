import type { ChildApplication } from '@/services/child-applications'
import type { DietNoteChangeRequest, MealPlan } from '@/services/meal'
import type { ParentOrganization } from '@/services/organizations'
import type { LeaveRequest, ParentChild, ParentHomework, ParentMe, ParentNotification, ParentPickupEvent, ParentPickupToday } from '@/services/parent'
import type { MessageSubscription, MessageSubscriptionKind } from '@/services/subscriptions'
import type { DailySummary } from '@/services/summary'
import { getStoredPhoneLoginPhone, saveAuthToken } from '@/services/auth'
import { createParentChildApplication, getParentChildApplications, updateParentChildApplication } from '@/services/child-applications'
import { clearPendingClassInviteToken, getPendingClassInviteToken, savePendingClassInviteToken } from '@/services/class-invites'
import { createParentDietNoteChangeRequest, getParentDietNote, getParentDietNoteChangeRequests, getParentMealHistory, getParentMeals, mealPhotoURL } from '@/services/meal'
import { getParentOrganizations, switchParentOrganization } from '@/services/organizations'
import { cancelParentLeaveRequest, createParentLeaveRequest, createParentPickupChange, getParentHomework, getParentLeaveRequests, getParentMe, getParentNotifications, getParentPickupEvents, getParentPickupToday, leaveStatusLabel, markParentNotificationRead, parentPhotoURL, updateParentLeaveRequest } from '@/services/parent'
import { getToday } from '@/services/pickup'
import { getParentPrivacyConsent, recordParentPrivacyConsent } from '@/services/privacy'
import { isRequestError } from '@/services/request'
import { getParentSubscriptions, hasConfiguredSubscriptionTemplates, requestParentSubscriptions } from '@/services/subscriptions'
import { getParentDailySummary, markParentDailySummaryRead } from '@/services/summary'
import { useAppStore } from '@/stores'
import { showFeedback } from '@/utils/feedback'
import { createLoadGuard } from '@/utils/load-guard'

const appStore = useAppStore()
const parentLoadGuard = createLoadGuard()
let selectedChildLoadSequence = 0

function nextDate(date: string) {
  const value = new Date(`${date}T00:00:00`)
  value.setDate(value.getDate() + 1)
  const month = `${value.getMonth() + 1}`.padStart(2, '0')
  const day = `${value.getDate()}`.padStart(2, '0')
  return `${value.getFullYear()}-${month}-${day}`
}

function decodeInviteToken(value: string | undefined) {
  if (!value) {
    return ''
  }
  try {
    return decodeURIComponent(value)
  }
  catch {
    return value
  }
}

function offsetDate(date: string, offset: number) {
  const value = new Date(`${date}T00:00:00`)
  value.setDate(value.getDate() + offset)
  const month = `${value.getMonth() + 1}`.padStart(2, '0')
  const day = `${value.getDate()}`.padStart(2, '0')
  return `${value.getFullYear()}-${month}-${day}`
}

function normalizePhone(value: string) {
  return value.replace(/\D/g, '')
}

type ParentPickupEventView = ParentPickupEvent & { photo_url_signed: string, status_label: string }
type ParentHomeworkView = ParentHomework & { attachment_urls_signed: string[], status_class: string, status_label: string }
type LeaveRequestView = LeaveRequest & { status_label: string }
type ParentPickupTodayView = ParentPickupToday & { status_label: string, class_label: string }
type ChildApplicationView = ChildApplication & { status_label: string }
type MealPlanView = MealPlan & { photo_url_signed: string }
type DietNoteChangeRequestView = DietNoteChangeRequest & { status_label: string }
type ParentSubscriptionView = MessageSubscription & { kind_label: string, status_label: string, status_class: string, detail: string }
type ParentFormField = 'childName' | 'schoolName' | 'classText' | 'guardianName' | 'guardianPhone' | 'relationship' | 'applicationNotes' | 'leaveDate' | 'leaveReason' | 'changeNote'
type ParentTab = 'home' | 'dynamic' | 'apply' | 'mine'
type ParentOrganizationView = ParentOrganization & { status_label: string, initial: string }

const applicationStatusLabels: Record<string, string> = {
  approved: '已通过',
  needs_info: '待补充资料',
  pending: '待老师审核',
  rejected: '未通过',
}

const statusLabels: Record<string, string> = {
  absent: '未到',
  abnormal: '异常',
  arrived: '已到托管班',
  leave: '请假',
  left: '已离班',
  midway_left: '中途离班',
  not_arrived: '到班异常',
  parent_picked_up: '家长接走',
  picked_up: '校门口接到',
  self_arrived: '自行到班',
}

function toPickupEventView(item: ParentPickupEvent): ParentPickupEventView {
  return { ...item, photo_url_signed: parentPhotoURL(item.photo_url), status_label: statusLabels[item.event_type] || item.event_type }
}

function homeworkStatusClass(status: ParentHomework['status']) {
  if (status === 'completed') {
    return 'status-badge-success'
  }
  if (status === 'incomplete' || status === 'not_submitted') {
    return 'status-badge-danger'
  }
  return ''
}

function toHomeworkView(item: ParentHomework): ParentHomeworkView {
  return {
    ...item,
    attachment_urls_signed: item.attachment_urls.map(url => parentPhotoURL(url)),
    status_class: homeworkStatusClass(item.status),
    status_label: ({ completed: '已完成', incomplete: '需订正', not_submitted: '未提交', pending: '待批改' })[item.status],
  }
}

function toLeaveView(item: LeaveRequest): LeaveRequestView {
  return { ...item, status_label: leaveStatusLabel(item.status) }
}

function toPickupTodayView(item: ParentPickupToday): ParentPickupTodayView {
  const labels: Record<string, string> = { absent: '未到', arrived: '已到托管班', confirmed: '今日已确认', draft: '待老师确认', finished: '已完成', leave: '请假', left: '已离班', midway_left: '中途离班', not_arrived: '到班异常', parent_picked_up: '家长接走', picked_up: '校门口接到', self_arrived: '自行到班', started: '接送中' }
  const classLabel = [item.school_name, `${item.grade || ''}${item.class_name || ''}`.trim()].filter(Boolean).join(' · ')
  return { ...item, status_label: labels[item.student_status] || labels[item.status] || item.student_status, class_label: classLabel || '班级待同步' } as ParentPickupTodayView
}

function childClassLabel(child?: ParentChild) {
  if (!child) {
    return '班级待同步'
  }
  const label = [child.school_name, `${child.grade || ''}${child.class_name || ''}`.trim()].filter(Boolean).join(' · ')
  return label || '班级待同步'
}

function toDietNoteChangeRequestView(item: DietNoteChangeRequest): DietNoteChangeRequestView {
  const statusLabels: Record<DietNoteChangeRequest['status'], string> = { pending: '待老师确认', approved: '已确认生效', rejected: '未通过' }
  return { ...item, status_label: statusLabels[item.status] || item.status }
}

const subscriptionKindLabels: Record<MessageSubscriptionKind, string> = {
  pickup: '接送提醒',
  meal: '餐食提醒',
  homework: '作业反馈',
  leave: '请假处理',
  summary: '每日总结',
}

const subscriptionStatusLabels: Record<string, string> = {
  accept: '已开启',
  reject: '未同意',
  ban: '已关闭',
  filter: '部分开启',
  unknown: '待授权',
}

function toSubscriptionView(item: MessageSubscription): ParentSubscriptionView {
  const status = item.status || 'unknown'
  const statusClass = status === 'accept' ? 'status-tone-success' : status === 'reject' || status === 'ban' ? 'status-tone-danger' : 'status-tone-warning'
  return {
    ...item,
    kind_label: subscriptionKindLabels[item.kind] || item.kind,
    status_label: subscriptionStatusLabels[status] || status,
    status_class: statusClass,
    detail: item.authorized_at ? `授权于 ${item.authorized_at}` : '点击上方按钮授权',
  }
}

function toOrganizationView(item: ParentOrganization): ParentOrganizationView {
  return { ...item, status_label: item.status === 'active' ? '正常使用' : '暂不可用', initial: item.name.slice(0, 1) || '机' }
}

Page({
  data: {
    contentReady: false,
    activeTab: 'home' as ParentTab,
    loading: false,
    applicationSubmitting: false,
    leaveSubmitting: false,
    pickupChangeSubmitting: false,
    bound: false,
    applications: [] as ChildApplicationView[],
    invitedSchoolClassID: 0,
    inviteToken: '',
    editingApplicationID: 0,
    editingSchoolClassID: 0,
    editingOriginalSchoolName: '',
    editingOriginalClassText: '',
    childName: '',
    schoolName: '',
    classText: '',
    guardianName: '',
    guardianPhone: '',
    relationship: '',
    focusedField: '' as ParentFormField | '',
    applicationNotes: '',
    children: [] as ParentChild[],
    selectedStudentID: 0,
    selectedStudentName: '',
    selectedStudentClassLabel: '班级待同步',
    pickupToday: null as ParentPickupTodayView | null,
    meals: [] as MealPlanView[],
    tomorrowMeals: [] as MealPlanView[],
    mealHistory: [] as MealPlanView[],
    dietNote: '',
    dietNoteSaving: false,
    dietNoteRequests: [] as DietNoteChangeRequestView[],
    dailySummary: null as DailySummary | null,
    dailySummaryChildUpdate: '',
    date: getToday(),
    tomorrowDate: nextDate(getToday()),
    events: [] as ParentPickupEventView[],
    notifications: [] as ParentNotification[],
    notificationUnreadCount: 0,
    notificationNextCursor: 0,
    notificationHasMore: false,
    notificationLoadingMore: false,
    homework: [] as ParentHomeworkView[],
    leaves: [] as LeaveRequestView[],
    leaveDate: getToday(),
    leaveReason: '',
    changeStatus: 'parent_picked_up' as 'parent_picked_up' | 'self_arrived' | 'leave' | 'absent',
    changeNote: '',
    subscriptionConfigured: hasConfiguredSubscriptionTemplates(),
    subscriptionLoading: false,
    subscriptions: [] as ParentSubscriptionView[],
    organizations: [] as ParentOrganizationView[],
    organizationSwitching: false,
    currentOrganizationName: '当前机构',
    privacyConsentVisible: false,
    privacyConsentLoading: false,
    privacyPolicyVersion: '',
    privacyConsentError: '',
    dynamicLoadNotice: '',
    redirectingToApply: false,
  },
  onLoad(options: Record<string, string | undefined> = {}) {
    const invitedSchoolClassID = Number(options.schoolClassId || 0)
    const inviteToken = decodeInviteToken(options.inviteToken || options.scene || getPendingClassInviteToken())
    this.setData({
      invitedSchoolClassID: Number.isFinite(invitedSchoolClassID) ? invitedSchoolClassID : 0,
      inviteToken,
      guardianPhone: getStoredPhoneLoginPhone(),
    })
    if (inviteToken) {
      savePendingClassInviteToken(inviteToken)
      if (appStore.authenticated && appStore.role === 'parent') {
        void this.joinOrganizationByInvite(inviteToken)
      }
    }
    void this.loadParentData()
  },
  onShow() {
    const pendingInviteToken = decodeInviteToken(getPendingClassInviteToken())
    if (pendingInviteToken && pendingInviteToken !== this.data.inviteToken && appStore.authenticated && appStore.role === 'parent') {
      this.setData({ inviteToken: pendingInviteToken })
      void this.joinOrganizationByInvite(pendingInviteToken)
    }
    void this.loadParentData()
  },
  async loadParentData() {
    return parentLoadGuard.run(() => this.loadParentDataInternal())
  },
  async loadParentDataInternal() {
    this.setData({ loading: true })
    try {
      const privacyAccepted = await this.loadPrivacyConsent()
      if (!privacyAccepted) {
        this.clearParentContent()
        this.setData({ loading: false })
        return
      }
      const me = await getParentMe()
      if (!(me.children || []).length) {
        if (this.data.redirectingToApply) {
          this.setData({ loading: false })
          return
        }
        parentLoadGuard.markDirty()
        this.clearParentContent()
        this.setData({ contentReady: false, loading: false, redirectingToApply: true })
        this.openAddChildPage(true)
        return
      }
      this.setData({ redirectingToApply: false })
      await this.applyParentMe(me)
      this.setData({ loading: false })
      void this.loadParentAccountSecondaryData()
    }
    catch (error) {
      if (isRequestError(error) && error.code === 'UNAUTHORIZED') {
        this.handleAuthExpired()
        this.setData({ loading: false })
        return
      }
      this.clearParentContent()
      parentLoadGuard.markDirty()
      this.showToast(error instanceof Error ? error.message : '家长数据加载失败，请重试')
      this.setData({ loading: false })
    }
  },
  async loadParentAccountSecondaryData() {
    try {
      const [subscriptionResult, applicationResult, organizationResult] = await Promise.allSettled([getParentSubscriptions(), getParentChildApplications(), getParentOrganizations()])
      if (subscriptionResult.status === 'fulfilled') {
        this.setData({ subscriptions: subscriptionResult.value.items.map(toSubscriptionView) })
      }
      if (applicationResult.status === 'fulfilled') {
        this.setData({ applications: applicationResult.value.items.map(item => ({ ...item, status_label: applicationStatusLabels[item.status] || item.status })) })
      }
      else {
        this.showToast(applicationResult.reason instanceof Error ? applicationResult.reason.message : '申请记录暂时无法加载')
      }
      if (organizationResult.status === 'fulfilled') {
        const organizations = organizationResult.value.items.map(toOrganizationView)
        this.setData({ organizations, currentOrganizationName: organizations.find(item => item.current)?.name || '当前机构' })
      }
    }
    catch {
      // Account-side secondary data must not hide the already loaded child timeline.
    }
  },
  clearParentContent() {
    this.setData({ bound: false, children: [], applications: [], events: [], notifications: [], notificationUnreadCount: 0, notificationNextCursor: 0, notificationHasMore: false, homework: [], leaves: [], pickupToday: null, meals: [], tomorrowMeals: [], mealHistory: [], dietNote: '', dietNoteRequests: [], dailySummary: null, dailySummaryChildUpdate: '', subscriptions: [], organizations: [], currentOrganizationName: '当前机构', selectedStudentClassLabel: '班级待同步', dynamicLoadNotice: '' })
  },
  async loadPrivacyConsent(): Promise<boolean> {
    this.setData({ privacyConsentLoading: true })
    try {
      const result = await getParentPrivacyConsent()
      this.setData({ privacyConsentVisible: !result.accepted, privacyPolicyVersion: result.current_policy_version || result.policy_version, privacyConsentError: '' })
      return result.accepted
    }
    catch (error) {
      // 登录状态失效时不能把家长困在隐私弹窗中，应回到登录入口重新建立会话。
      if (isRequestError(error) && error.code === 'UNAUTHORIZED') {
        this.handleAuthExpired()
        return false
      }
      // 隐私状态无法确认时保持遮罩，避免在合规状态未知时继续查看儿童信息。
      this.setData({ privacyConsentVisible: true, privacyConsentError: '隐私说明暂时无法加载，确认成功前不能查看孩子动态。' })
      return false
    }
    finally {
      this.setData({ privacyConsentLoading: false })
    }
  },
  async handleRetryPrivacyConsent() {
    if (this.data.privacyConsentLoading) {
      return
    }
    parentLoadGuard.markDirty()
    const accepted = await this.loadPrivacyConsent()
    if (accepted) {
      parentLoadGuard.markDirty()
      void this.loadParentData()
    }
  },
  handlePrivacyBackToLogin() {
    this.handleAuthExpired()
  },
  async handleAcceptPrivacyConsent() {
    if (this.data.privacyConsentLoading || !this.data.privacyPolicyVersion) {
      return
    }
    this.setData({ privacyConsentLoading: true })
    try {
      await recordParentPrivacyConsent(this.data.privacyPolicyVersion)
      this.setData({ privacyConsentVisible: false })
      this.showToast('已同意隐私说明')
      parentLoadGuard.markDirty()
      void this.loadParentData()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '隐私确认失败，请重试')
    }
    finally {
      this.setData({ privacyConsentLoading: false })
    }
  },
  async applyParentMe(me: ParentMe) {
    const children = me.children || []
    const selectedStudentID = this.data.selectedStudentID && children.some(item => item.student_id === this.data.selectedStudentID) ? this.data.selectedStudentID : (children[0]?.student_id || 0)
    const selectedChild = children.find(item => item.student_id === selectedStudentID)
    this.setData({ bound: children.length > 0, children, selectedStudentID, selectedStudentName: selectedChild?.student_name || '', selectedStudentClassLabel: childClassLabel(selectedChild), contentReady: true })
    if (!selectedStudentID) {
      this.setData({ events: [], notifications: [], notificationUnreadCount: 0, notificationNextCursor: 0, notificationHasMore: false, homework: [], leaves: [], pickupToday: null, meals: [], tomorrowMeals: [], mealHistory: [], dietNote: '', dietNoteRequests: [], dailySummary: null, dailySummaryChildUpdate: '', dynamicLoadNotice: '' })
      return
    }
    await this.loadSelectedChildData(selectedStudentID)
  },
  async loadSelectedChildData(studentID: number) {
    const loadSequence = ++selectedChildLoadSequence
    const isCurrent = () => loadSequence === selectedChildLoadSequence && this.data.selectedStudentID === studentID
    this.setData({ dynamicLoadNotice: '正在同步孩子的最新动态…' })
    const criticalResults = await Promise.allSettled([
      getParentPickupToday(studentID, this.data.date),
      getParentPickupEvents(studentID, this.data.date),
      getParentNotifications({ limit: 20 }),
      getParentMeals(this.data.date),
      getParentHomework(studentID),
      getParentDailySummary(this.data.date),
    ])
    if (!isCurrent()) {
      return
    }
    const [pickupTodayResult, eventsResult, notificationsResult, mealsResult, homeworkResult, dailySummaryResult] = criticalResults
    const pickupToday = pickupTodayResult.status === 'fulfilled' ? pickupTodayResult.value : null
    const events = eventsResult.status === 'fulfilled' ? eventsResult.value : { items: [] }
    const notifications = notificationsResult.status === 'fulfilled' ? notificationsResult.value : { items: [], unread: 0, next_cursor: 0 }
    const meals = mealsResult.status === 'fulfilled' ? mealsResult.value : { items: [] }
    const homework = homeworkResult.status === 'fulfilled' ? homeworkResult.value : { items: [] }
    const dailySummaryValue = dailySummaryResult.status === 'fulfilled' ? dailySummaryResult.value : null
    if (dailySummaryValue && !dailySummaryValue.read_at) {
      void markParentDailySummaryRead(dailySummaryValue.id).catch(() => undefined)
      dailySummaryValue.read_at = new Date().toISOString()
    }
    const criticalFailed = criticalResults.some(result => result.status === 'rejected')
    this.setData({ events: events.items.map(toPickupEventView), notifications: notifications.items, notificationUnreadCount: notifications.unread, notificationNextCursor: notifications.next_cursor, notificationHasMore: notifications.next_cursor > 0, homework: homework.items.map(toHomeworkView), pickupToday: pickupToday ? toPickupTodayView(pickupToday) : null, meals: meals.items.map(item => ({ ...item, photo_url_signed: mealPhotoURL(item.photo_url) })), dailySummary: dailySummaryValue, dailySummaryChildUpdate: dailySummaryValue?.child_updates?.[String(studentID)] || '', dynamicLoadNotice: criticalFailed ? '部分今日动态暂时加载失败，请下拉或重新进入页面。' : '正在补充历史记录和家庭安排…' })

    void this.loadSelectedChildSecondaryData(studentID, loadSequence, criticalFailed)
  },
  async loadSelectedChildSecondaryData(studentID: number, loadSequence: number, criticalFailed: boolean) {
    const isCurrent = () => loadSequence === selectedChildLoadSequence && this.data.selectedStudentID === studentID
    const results = await Promise.allSettled([
      getParentLeaveRequests(),
      getParentMeals(this.data.tomorrowDate),
      getParentDietNote(studentID),
      getParentMealHistory(offsetDate(this.data.date, -6), this.data.date),
      getParentDietNoteChangeRequests(studentID),
    ])
    if (!isCurrent()) {
      return
    }
    const [leavesResult, tomorrowMealsResult, dietNoteResult, mealHistoryResult, dietNoteRequestsResult] = results
    const leaves = leavesResult.status === 'fulfilled' ? leavesResult.value : { items: [] }
    const tomorrowMeals = tomorrowMealsResult.status === 'fulfilled' ? tomorrowMealsResult.value : { items: [] }
    const dietNote = dietNoteResult.status === 'fulfilled' ? dietNoteResult.value : null
    const mealHistory = mealHistoryResult.status === 'fulfilled' ? mealHistoryResult.value : { items: [] }
    const dietNoteRequests = dietNoteRequestsResult.status === 'fulfilled' ? dietNoteRequestsResult.value : { items: [] }
    const hasFailed = results.some(result => result.status === 'rejected')
    this.setData({ leaves: leaves.items.map(toLeaveView), tomorrowMeals: tomorrowMeals.items.map(item => ({ ...item, photo_url_signed: mealPhotoURL(item.photo_url) })), mealHistory: mealHistory.items.map(item => ({ ...item, photo_url_signed: mealPhotoURL(item.photo_url) })), dietNote: dietNote?.note || '', dietNoteRequests: dietNoteRequests.items.map(toDietNoteChangeRequestView), dynamicLoadNotice: criticalFailed ? '部分今日动态暂时加载失败，请下拉或重新进入页面。' : hasFailed ? '部分历史记录暂时加载失败，请稍后重试。' : '' })
  },
  handleInput(event: WechatMiniprogram.Input) {
    const field = event.currentTarget.dataset.field as ParentFormField
    this.setData({ [field]: event.detail.value })
  },
  handleTabChange(event: WechatMiniprogram.TouchEvent) {
    const tab = event.currentTarget.dataset.tab as ParentTab
    if (tab === 'apply') {
      this.openAddChildPage()
      return
    }
    if (['home', 'dynamic', 'apply', 'mine'].includes(tab)) {
      this.setData({ activeTab: tab })
    }
  },
  openAddChildPage(replace = false) {
    if (typeof wx === 'undefined') {
      this.setData({ activeTab: 'apply', contentReady: true, redirectingToApply: false })
      return
    }
    const inviteQuery = this.data.inviteToken
      ? `?inviteToken=${encodeURIComponent(this.data.inviteToken)}`
      : this.data.invitedSchoolClassID
        ? `?schoolClassId=${this.data.invitedSchoolClassID}`
        : ''
    const payload = {
      url: `/pages/parent-apply/index${inviteQuery}`,
      fail: () => this.setData({ activeTab: 'apply', contentReady: true, redirectingToApply: false }),
    }
    if (replace) {
      wx.redirectTo(payload)
      return
    }
    wx.navigateTo(payload)
  },
  handleBackToIdentity() {
    if (typeof wx !== 'undefined') {
      wx.reLaunch({ url: '/pages/index/index' })
    }
  },
  handleOpenWrongbook() {
    if (!this.data.selectedStudentID) {
      this.showToast('请先选择孩子')
      return
    }
    if (typeof wx !== 'undefined') {
      wx.navigateTo({ url: `/pages/wrongbook/index?mode=parent&studentId=${this.data.selectedStudentID}` })
    }
  },
  handleFocus(event: WechatMiniprogram.InputFocus) {
    const field = event.currentTarget.dataset.field as ParentFormField
    this.setData({ focusedField: field })
  },
  handleBlur(event: WechatMiniprogram.InputBlur) {
    const field = event.currentTarget.dataset.field as ParentFormField
    if (this.data.focusedField === field) {
      this.setData({ focusedField: '' })
    }
  },
  async handleSubmitApplication() {
    if (this.data.applicationSubmitting) {
      return
    }
    const childName = this.data.childName.trim()
    const guardianPhone = normalizePhone(this.data.guardianPhone.trim())
    if (!childName) {
      this.showToast('请填写孩子姓名')
      return
    }
    if (!guardianPhone) {
      this.showToast('请填写家长手机号')
      return
    }
    if (guardianPhone.length < 7) {
      this.showToast('请输入有效的家长手机号')
      return
    }
    if (this.data.inviteToken && !this.data.invitedSchoolClassID) {
      this.showToast('班级邀请还在确认，请稍候')
      return
    }
    const retainsExistingClass = Boolean(
      this.data.editingApplicationID
      && this.data.schoolName.trim() === this.data.editingOriginalSchoolName
      && this.data.classText.trim() === this.data.editingOriginalClassText,
    )
    const schoolClassID = this.data.invitedSchoolClassID || (retainsExistingClass ? this.data.editingSchoolClassID : 0)
    this.setData({ applicationSubmitting: true })
    try {
      const payload = {
        student_name: childName,
        school_name: this.data.schoolName.trim(),
        class_text: this.data.classText.trim(),
        ...(schoolClassID ? { school_class_id: schoolClassID } : {}),
        ...(this.data.inviteToken ? { invite_token: this.data.inviteToken } : {}),
        guardian_name: this.data.guardianName.trim(),
        guardian_phone: guardianPhone,
        relationship: this.data.relationship.trim() || '家长',
        notes: this.data.applicationNotes.trim(),
      }
      if (this.data.editingApplicationID) {
        await updateParentChildApplication(this.data.editingApplicationID, payload)
      }
      else {
        await createParentChildApplication(payload)
      }
      this.showToast(this.data.editingApplicationID ? '补充资料已提交，等待老师审核' : '申请已提交，等待老师审核')
      this.setData({ editingApplicationID: 0, editingSchoolClassID: 0, editingOriginalSchoolName: '', editingOriginalClassText: '', childName: '', schoolName: '', classText: '', guardianName: '', guardianPhone, relationship: '', applicationNotes: '', focusedField: '' })
      if (this.data.inviteToken) {
        clearPendingClassInviteToken()
      }
      parentLoadGuard.markDirty()
      await this.loadParentData()
    }
    catch (error) {
      if (isRequestError(error) && error.code === 'UNAUTHORIZED') {
        this.handleAuthExpired()
        return
      }
      this.showToast(error instanceof Error ? error.message : '绑定失败')
    }
    finally {
      this.setData({ applicationSubmitting: false })
    }
  },
  handleResubmitApplication(event: WechatMiniprogram.TouchEvent) {
    const applicationID = Number(event.currentTarget.dataset.applicationId)
    const application = this.data.applications.find(item => item.id === applicationID)
    if (!application) {
      return
    }
    this.setData({
      editingApplicationID: application.id,
      editingSchoolClassID: application.school_class_id || 0,
      childName: application.student_name,
      schoolName: application.school_name_input,
      classText: application.grade_input || application.grade || [application.grade_input, application.class_name_input].filter(Boolean).join('') || application.class_name,
      guardianName: application.guardian_name,
      guardianPhone: application.guardian_phone || getStoredPhoneLoginPhone(),
      relationship: application.relationship || '',
      applicationNotes: application.notes,
      editingOriginalSchoolName: application.school_name_input,
      editingOriginalClassText: [application.grade_input, application.class_name_input].filter(Boolean).join('') || application.grade || application.class_name,
    })
    this.showToast('已带入原申请，请补充资料后重新提交')
  },
  async handleSelectChild(event: WechatMiniprogram.TouchEvent) {
    const studentID = Number(event.currentTarget.dataset.studentId)
    const child = this.data.children.find(item => item.student_id === studentID)
    if (!child) {
      return
    }
    this.setData({ selectedStudentID: studentID, selectedStudentName: child.student_name, selectedStudentClassLabel: childClassLabel(child), loading: true })
    try {
      await this.loadSelectedChildData(studentID)
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '孩子动态加载失败')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  async handleSubmitLeave() {
    if (this.data.leaveSubmitting) {
      return
    }
    if (!this.data.selectedStudentID || !this.data.leaveDate || !this.data.leaveReason.trim()) {
      this.showToast('请填写请假日期和原因')
      return
    }
    this.setData({ leaveSubmitting: true })
    try {
      await createParentLeaveRequest(this.data.selectedStudentID, { leave_date: this.data.leaveDate, reason: this.data.leaveReason.trim() })
      this.showToast('请假已提交，等待老师确认')
      this.setData({ leaveReason: '' })
      const leaves = await getParentLeaveRequests()
      this.setData({ leaves: leaves.items.map(toLeaveView) })
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '请假提交失败')
    }
    finally {
      this.setData({ leaveSubmitting: false })
    }
  },
  async handleEditLeave(event: WechatMiniprogram.TouchEvent) {
    const leaveID = Number(event.currentTarget.dataset.leaveId)
    const leave = this.data.leaves.find(item => item.id === leaveID)
    if (!leave || leave.status !== 'pending' || this.data.loading) {
      return
    }
    if (typeof wx === 'undefined') {
      return
    }
    wx.showModal({
      title: '修改请假原因',
      editable: true,
      content: leave.reason,
      placeholderText: '请填写请假原因',
      success: (result) => {
        if (!result.confirm || !result.content?.trim()) {
          return
        }
        void this.updateLeave(leave, result.content.trim())
      },
    })
  },
  async updateLeave(leave: LeaveRequestView, reason: string) {
    if (this.data.leaveSubmitting) {
      return
    }
    this.setData({ leaveSubmitting: true })
    try {
      const updated = await updateParentLeaveRequest(leave.id, { leave_date: leave.leave_date, reason })
      this.setData({ leaves: this.data.leaves.map(item => item.id === updated.id ? toLeaveView(updated) : item) })
      this.showToast('请假申请已修改，等待老师确认')
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '请假修改失败')
    }
    finally {
      this.setData({ leaveSubmitting: false })
    }
  },
  async handleCancelLeave(event: WechatMiniprogram.TouchEvent) {
    const leaveID = Number(event.currentTarget.dataset.leaveId)
    const leave = this.data.leaves.find(item => item.id === leaveID)
    if (!leave || leave.status !== 'pending' || this.data.leaveSubmitting) {
      return
    }
    const cancel = () => {
      this.setData({ leaveSubmitting: true })
      void cancelParentLeaveRequest(leaveID).then((updated) => {
        this.setData({ leaves: this.data.leaves.map(item => item.id === updated.id ? toLeaveView(updated) : item) })
        this.showToast('请假申请已撤回')
      }).catch(error => this.showToast(error instanceof Error ? error.message : '请假撤回失败')).finally(() => this.setData({ leaveSubmitting: false }))
    }
    if (typeof wx === 'undefined') {
      cancel()
      return
    }
    wx.showModal({ title: '撤回请假', content: '撤回后老师将不再按这条申请处理，确定撤回吗？', success: (result) => {
      if (result.confirm) {
        cancel()
      }
    } })
  },
  async handleLoadMoreNotifications() {
    if (!this.data.notificationHasMore || this.data.notificationLoadingMore || !this.data.notificationNextCursor) {
      return
    }
    this.setData({ notificationLoadingMore: true })
    try {
      const result = await getParentNotifications({ limit: 20, cursor: this.data.notificationNextCursor })
      const existing = new Set(this.data.notifications.map(item => item.id))
      const additions = result.items.filter(item => !existing.has(item.id))
      this.setData({ notifications: [...this.data.notifications, ...additions], notificationUnreadCount: result.unread, notificationNextCursor: result.next_cursor, notificationHasMore: result.next_cursor > 0 })
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '更多消息加载失败')
    }
    finally {
      this.setData({ notificationLoadingMore: false })
    }
  },
  handleSetChangeStatus(event: WechatMiniprogram.TouchEvent) {
    this.setData({ changeStatus: event.currentTarget.dataset.status })
  },
  async handleSubmitPickupChange() {
    if (this.data.pickupChangeSubmitting) {
      return
    }
    if (!this.data.selectedStudentID || !this.data.changeNote.trim()) {
      this.showToast('请填写临时接送说明')
      return
    }
    this.setData({ pickupChangeSubmitting: true })
    try {
      await createParentPickupChange(this.data.selectedStudentID, { change_date: this.data.date, requested_status: this.data.changeStatus, note: this.data.changeNote.trim() })
      this.showToast('临时变更已提交，老师会在工作台确认')
      this.setData({ changeNote: '' })
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '临时变更提交失败')
    }
    finally {
      this.setData({ pickupChangeSubmitting: false })
    }
  },
  handleEditDietNote() {
    if (!this.data.selectedStudentID || this.data.dietNoteSaving || typeof wx === 'undefined') {
      return
    }
    if (this.data.dietNoteRequests.some(item => item.status === 'pending')) {
      this.showToast('已有饮食备注变更在等待老师确认')
      return
    }
    wx.showModal({
      title: '饮食和过敏备注',
      editable: true,
      content: this.data.dietNote,
      placeholderText: '例如：花生过敏、忌牛奶；没有可留空',
      success: (result) => {
        if (!result.confirm) {
          return
        }
        void this.saveDietNote((result.content || '').trim())
      },
    })
  },
  async saveDietNote(note: string) {
    this.setData({ dietNoteSaving: true })
    try {
      const request = await createParentDietNoteChangeRequest(this.data.selectedStudentID, note)
      this.setData({ dietNoteRequests: [toDietNoteChangeRequestView(request), ...this.data.dietNoteRequests] })
      this.showToast('变更已提交，老师确认后生效')
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '饮食备注保存失败')
    }
    finally {
      this.setData({ dietNoteSaving: false })
    }
  },
  async handleMarkNotificationRead(event: WechatMiniprogram.TouchEvent) {
    const id = Number(event.currentTarget.dataset.notificationId)
    if (!id) {
      return
    }
    try {
      const notification = this.data.notifications.find(item => item.id === id)
      await markParentNotificationRead(id)
      this.setData({ notifications: this.data.notifications.map(item => item.id === id ? { ...item, read_at: item.read_at || new Date().toISOString() } : item), notificationUnreadCount: Math.max(0, this.data.notificationUnreadCount - (notification?.read_at ? 0 : 1)) })
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '消息操作失败')
    }
  },
  async handleEnableNotifications() {
    if (!this.data.subscriptionConfigured) {
      this.showToast('管理员还没有配置微信通知模板')
      return
    }
    this.setData({ subscriptionLoading: true })
    try {
      const subscriptions = await requestParentSubscriptions()
      this.setData({ subscriptions: subscriptions.map(toSubscriptionView) })
      const accepted = subscriptions.filter(item => item.status === 'accept').length
      const total = subscriptions.length
      this.showToast(accepted ? `已开启 ${accepted}/${total} 类微信通知` : '未完成微信授权，站内通知仍可正常查看')
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '通知授权未完成')
    }
    finally {
      this.setData({ subscriptionLoading: false })
    }
  },
  async joinOrganizationByInvite(inviteToken: string) {
    if (!inviteToken || this.data.organizationSwitching) {
      return
    }
    this.setData({ organizationSwitching: true })
    try {
      const token = await switchParentOrganization({ invite_token: inviteToken })
      saveAuthToken(token)
      if (inviteToken.startsWith('o')) {
        clearPendingClassInviteToken()
      }
      parentLoadGuard.markDirty()
      this.showToast('已加入并切换到新机构')
      await this.loadParentData()
      const organizationResult = await getParentOrganizations()
      const organizations = organizationResult.items.map(toOrganizationView)
      this.setData({ organizations, currentOrganizationName: organizations.find(item => item.current)?.name || '当前机构' })
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '加入机构失败，请确认二维码有效')
    }
    finally {
      this.setData({ organizationSwitching: false })
    }
  },
  async handleSwitchOrganization(event: WechatMiniprogram.TouchEvent) {
    const organizationID = Number(event.currentTarget.dataset.organizationId)
    if (!organizationID || this.data.organizationSwitching || this.data.organizations.find(item => item.id === organizationID)?.current) {
      return
    }
    this.setData({ organizationSwitching: true })
    try {
      const token = await switchParentOrganization({ organization_id: organizationID })
      saveAuthToken(token)
      parentLoadGuard.markDirty()
      const organizationResult = await getParentOrganizations()
      const organizations = organizationResult.items.map(toOrganizationView)
      this.setData({ organizations, currentOrganizationName: organizations.find(item => item.current)?.name || '当前机构' })
      await this.loadParentData()
      this.showToast('已切换机构')
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '机构切换失败，请稍后重试')
    }
    finally {
      this.setData({ organizationSwitching: false })
    }
  },
  handleAuthExpired() {
    appStore.clearAuthenticated()
    if (typeof wx !== 'undefined') {
      wx.reLaunch({ url: '/pages/index/index' })
    }
  },
  eventStatusLabel(status: string) { return statusLabels[status] || status },
  homeworkStatusLabel(status: ParentHomework['status']) { return ({ completed: '已完成', incomplete: '需订正', not_submitted: '未提交', pending: '待批改' })[status] },
  leaveStatusLabel,
  parentPhotoURL,
  showToast(message: string) { showFeedback(this, message) },
})
