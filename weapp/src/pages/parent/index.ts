import type { ChildApplication } from '@/services/child-applications'
import type { DietNoteChangeRequest, MealPlan } from '@/services/meal'
import type { LeaveRequest, ParentChild, ParentHomework, ParentMe, ParentNotification, ParentPickupEvent, ParentPickupToday } from '@/services/parent'
import type { MessageSubscription, MessageSubscriptionKind } from '@/services/subscriptions'
import type { DailySummary } from '@/services/summary'
import { getStoredPhoneLoginPhone } from '@/services/auth'
import { createParentChildApplication, getParentChildApplications, updateParentChildApplication } from '@/services/child-applications'
import { createParentDietNoteChangeRequest, getParentDietNote, getParentDietNoteChangeRequests, getParentMealHistory, getParentMeals, mealPhotoURL } from '@/services/meal'
import { cancelParentLeaveRequest, createParentLeaveRequest, createParentPickupChange, getParentHomework, getParentLeaveRequests, getParentMe, getParentNotifications, getParentPickupEvents, getParentPickupToday, leaveStatusLabel, markParentNotificationRead, parentPhotoURL, updateParentLeaveRequest } from '@/services/parent'
import { getToday } from '@/services/pickup'
import { getParentPrivacyConsent, recordParentPrivacyConsent } from '@/services/privacy'
import { isRequestError } from '@/services/request'
import { getParentSubscriptions, hasConfiguredSubscriptionTemplates, requestParentSubscriptions } from '@/services/subscriptions'
import { getParentDailySummary, markParentDailySummaryRead } from '@/services/summary'
import { useAppStore } from '@/stores'
import { showFeedback } from '@/utils/feedback'

const appStore = useAppStore()

function nextDate(date: string) {
  const value = new Date(`${date}T00:00:00`)
  value.setDate(value.getDate() + 1)
  const month = `${value.getMonth() + 1}`.padStart(2, '0')
  const day = `${value.getDate()}`.padStart(2, '0')
  return `${value.getFullYear()}-${month}-${day}`
}

function offsetDate(date: string, offset: number) {
  const value = new Date(`${date}T00:00:00`)
  value.setDate(value.getDate() + offset)
  const month = `${value.getMonth() + 1}`.padStart(2, '0')
  const day = `${value.getDate()}`.padStart(2, '0')
  return `${value.getFullYear()}-${month}-${day}`
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

Page({
  data: {
    contentReady: false,
    activeTab: 'home' as ParentTab,
    loading: false,
    bound: false,
    applications: [] as ChildApplicationView[],
    invitedSchoolClassID: 0,
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
    privacyConsentVisible: false,
    privacyConsentLoading: false,
    privacyPolicyVersion: '',
    privacyConsentError: '',
    dynamicLoadNotice: '',
  },
  onLoad(options: Record<string, string | undefined> = {}) {
    const invitedSchoolClassID = Number(options.schoolClassId || 0)
    this.setData({
      invitedSchoolClassID: Number.isFinite(invitedSchoolClassID) ? invitedSchoolClassID : 0,
      guardianPhone: getStoredPhoneLoginPhone(),
    })
    void this.loadParentData()
  },
  onShow() {
    void this.loadParentData()
  },
  async loadParentData() {
    this.setData({ loading: true })
    try {
      const privacyAccepted = await this.loadPrivacyConsent()
      if (!privacyAccepted) {
        this.clearParentContent()
        return
      }
      const me = await getParentMe()
      if (!(me.children || []).length) {
        this.clearParentContent()
        this.setData({ contentReady: false, loading: false })
        this.openAddChildPage(true)
        return
      }
      await this.applyParentMe(me)
    }
    catch (error) {
      if (isRequestError(error) && error.code === 'UNAUTHORIZED') {
        this.handleAuthExpired()
        return
      }
      this.clearParentContent()
      this.showToast(error instanceof Error ? error.message : '家长数据加载失败，请重试')
      return
    }
    await this.loadSubscriptionStatus()
    try {
      const applications = await getParentChildApplications()
      this.setData({ applications: applications.items.map(item => ({ ...item, status_label: applicationStatusLabels[item.status] || item.status })) })
    }
    catch (error) {
      this.setData({ applications: [] })
      this.showToast(error instanceof Error ? error.message : '申请记录暂时无法加载')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  clearParentContent() {
    this.setData({ bound: false, children: [], applications: [], events: [], notifications: [], notificationUnreadCount: 0, notificationNextCursor: 0, notificationHasMore: false, homework: [], leaves: [], pickupToday: null, meals: [], tomorrowMeals: [], mealHistory: [], dietNote: '', dietNoteRequests: [], dailySummary: null, dailySummaryChildUpdate: '', subscriptions: [], selectedStudentClassLabel: '班级待同步', dynamicLoadNotice: '' })
  },
  async loadSubscriptionStatus() {
    try {
      const result = await getParentSubscriptions()
      this.setData({ subscriptions: result.items.map(toSubscriptionView) })
    }
    catch {
      // 授权状态不是业务主数据，加载失败时保留站内消息和孩子动态。
      this.setData({ subscriptions: [] })
    }
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
    await this.loadPrivacyConsent()
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
    const results = await Promise.allSettled([getParentPickupEvents(studentID, this.data.date), getParentNotifications({ limit: 20 }), getParentHomework(studentID), getParentLeaveRequests(), getParentPickupToday(studentID, this.data.date), getParentMeals(this.data.date), getParentMeals(this.data.tomorrowDate), getParentDietNote(studentID), getParentDailySummary(this.data.date), getParentMealHistory(offsetDate(this.data.date, -6), this.data.date), getParentDietNoteChangeRequests(studentID)])
    const [eventsResult, notificationsResult, homeworkResult, leavesResult, pickupTodayResult, mealsResult, tomorrowMealsResult, dietNoteResult, dailySummaryResult, mealHistoryResult, dietNoteRequestsResult] = results
    const events = eventsResult.status === 'fulfilled' ? eventsResult.value : { items: [] }
    const notifications = notificationsResult.status === 'fulfilled' ? notificationsResult.value : { items: [], unread: 0, next_cursor: 0 }
    const homework = homeworkResult.status === 'fulfilled' ? homeworkResult.value : { items: [] }
    const leaves = leavesResult.status === 'fulfilled' ? leavesResult.value : { items: [] }
    const pickupToday = pickupTodayResult.status === 'fulfilled' ? pickupTodayResult.value : null
    const meals = mealsResult.status === 'fulfilled' ? mealsResult.value : { items: [] }
    const tomorrowMeals = tomorrowMealsResult.status === 'fulfilled' ? tomorrowMealsResult.value : { items: [] }
    const dietNote = dietNoteResult.status === 'fulfilled' ? dietNoteResult.value : null
    const dailySummaryValue = dailySummaryResult.status === 'fulfilled' ? dailySummaryResult.value : null
    const mealHistory = mealHistoryResult.status === 'fulfilled' ? mealHistoryResult.value : { items: [] }
    const dietNoteRequests = dietNoteRequestsResult.status === 'fulfilled' ? dietNoteRequestsResult.value : { items: [] }
    const dailySummary = dailySummaryValue
    if (dailySummary && !dailySummary.read_at) {
      void markParentDailySummaryRead(dailySummary.id).catch(() => undefined)
      dailySummary.read_at = new Date().toISOString()
    }
    const hasFailed = results.some(result => result.status === 'rejected')
    this.setData({ events: events.items.map(toPickupEventView), notifications: notifications.items, notificationUnreadCount: notifications.unread, notificationNextCursor: notifications.next_cursor, notificationHasMore: notifications.next_cursor > 0, homework: homework.items.map(toHomeworkView), leaves: leaves.items.map(toLeaveView), pickupToday: pickupToday ? toPickupTodayView(pickupToday) : null, meals: meals.items.map(item => ({ ...item, photo_url_signed: mealPhotoURL(item.photo_url) })), tomorrowMeals: tomorrowMeals.items.map(item => ({ ...item, photo_url_signed: mealPhotoURL(item.photo_url) })), mealHistory: mealHistory.items.map(item => ({ ...item, photo_url_signed: mealPhotoURL(item.photo_url) })), dietNote: dietNote?.note || '', dietNoteRequests: dietNoteRequests.items.map(toDietNoteChangeRequestView), dailySummary: dailySummaryValue, dailySummaryChildUpdate: dailySummaryValue?.child_updates?.[String(studentID)] || '', dynamicLoadNotice: hasFailed ? '部分动态暂时加载失败，请稍后重新进入页面。' : '' })
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
      this.setData({ activeTab: 'apply', contentReady: true })
      return
    }
    const payload = {
      url: '/pages/parent-apply/index',
      fail: () => this.setData({ activeTab: 'apply', contentReady: true }),
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
    const childName = this.data.childName.trim()
    const guardianPhone = this.data.guardianPhone.trim()
    if (!childName || !guardianPhone) {
      this.showToast('请填写孩子姓名和家长手机号')
      return
    }
    const retainsExistingClass = Boolean(
      this.data.editingApplicationID
      && this.data.schoolName.trim() === this.data.editingOriginalSchoolName
      && this.data.classText.trim() === this.data.editingOriginalClassText,
    )
    const schoolClassID = this.data.invitedSchoolClassID || (retainsExistingClass ? this.data.editingSchoolClassID : 0)
    if (!schoolClassID && !this.data.classText.trim()) {
      this.showToast('请填写孩子年级')
      return
    }
    this.setData({ loading: true })
    try {
      const payload = {
        student_name: childName,
        school_name: this.data.schoolName.trim(),
        grade: this.data.classText.trim(),
        ...(schoolClassID ? { school_class_id: schoolClassID } : {}),
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
      this.setData({ editingApplicationID: 0, editingSchoolClassID: 0, editingOriginalSchoolName: '', editingOriginalClassText: '', childName: '', schoolName: '', classText: '', guardianName: '', guardianPhone: getStoredPhoneLoginPhone(), relationship: '', applicationNotes: '', focusedField: '' })
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
      this.setData({ loading: false })
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
    if (!this.data.selectedStudentID || !this.data.leaveDate || !this.data.leaveReason.trim()) {
      this.showToast('请填写请假日期和原因')
      return
    }
    this.setData({ loading: true })
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
      this.setData({ loading: false })
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
    this.setData({ loading: true })
    try {
      const updated = await updateParentLeaveRequest(leave.id, { leave_date: leave.leave_date, reason })
      this.setData({ leaves: this.data.leaves.map(item => item.id === updated.id ? toLeaveView(updated) : item) })
      this.showToast('请假申请已修改，等待老师确认')
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '请假修改失败')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  async handleCancelLeave(event: WechatMiniprogram.TouchEvent) {
    const leaveID = Number(event.currentTarget.dataset.leaveId)
    const leave = this.data.leaves.find(item => item.id === leaveID)
    if (!leave || leave.status !== 'pending' || this.data.loading) {
      return
    }
    const cancel = () => {
      this.setData({ loading: true })
      void cancelParentLeaveRequest(leaveID).then((updated) => {
        this.setData({ leaves: this.data.leaves.map(item => item.id === updated.id ? toLeaveView(updated) : item) })
        this.showToast('请假申请已撤回')
      }).catch(error => this.showToast(error instanceof Error ? error.message : '请假撤回失败')).finally(() => this.setData({ loading: false }))
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
    if (!this.data.selectedStudentID || !this.data.changeNote.trim()) {
      this.showToast('请填写临时接送说明')
      return
    }
    this.setData({ loading: true })
    try {
      await createParentPickupChange(this.data.selectedStudentID, { change_date: this.data.date, requested_status: this.data.changeStatus, note: this.data.changeNote.trim() })
      this.showToast('临时变更已提交，老师会在工作台确认')
      this.setData({ changeNote: '' })
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '临时变更提交失败')
    }
    finally {
      this.setData({ loading: false })
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
