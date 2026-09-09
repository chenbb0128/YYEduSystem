import type { ChildApplication } from '@/services/child-applications'
import { getStoredPhoneLoginPhone, saveAuthToken } from '@/services/auth'
import { createParentChildApplication, getParentChildApplications, updateParentChildApplication } from '@/services/child-applications'
import { clearPendingClassInviteToken, getClassInvite, getPendingClassInviteToken } from '@/services/class-invites'
import { switchParentOrganization } from '@/services/organizations'
import { getParentMe } from '@/services/parent'
import { isRequestError } from '@/services/request'
import { useAppStore } from '@/stores'
import { showFeedback } from '@/utils/feedback'
import { createLoadGuard } from '@/utils/load-guard'

const appStore = useAppStore()
const applicationLoadGuard = createLoadGuard()

type ParentFormField = 'childName' | 'schoolName' | 'classText' | 'guardianName' | 'guardianPhone' | 'relationship' | 'applicationNotes'

interface ChildApplicationView extends ChildApplication {
  status_label: string
}

const applicationStatusLabels: Record<string, string> = {
  approved: '已通过',
  needs_info: '待补充资料',
  pending: '待老师审核',
  rejected: '未通过',
}

function toApplicationView(item: ChildApplication): ChildApplicationView {
  return { ...item, status_label: applicationStatusLabels[item.status] || item.status }
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

function normalizePhone(value: string) {
  return value.replace(/\D/g, '')
}

Page({
  data: {
    loading: false,
    submitting: false,
    hasBoundChild: false,
    applications: [] as ChildApplicationView[],
    invitedSchoolClassID: 0,
    inviteToken: '',
    inviteType: 'class' as 'class' | 'organization',
    organizationSwitching: false,
    inviteClassLabel: '',
    inviteLoading: false,
    inviteError: '',
    isInvited: false,
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
    optionalVisible: false,
    focusedField: '' as ParentFormField | '',
    applicationNotes: '',
  },
  onLoad(options: Record<string, string | undefined> = {}) {
    const invitedSchoolClassID = Number(options.schoolClassId || 0)
    const inviteToken = decodeInviteToken(options.inviteToken || options.scene || getPendingClassInviteToken())
    this.setData({
      invitedSchoolClassID: Number.isFinite(invitedSchoolClassID) ? invitedSchoolClassID : 0,
      inviteToken,
      isInvited: Boolean(invitedSchoolClassID || inviteToken),
      guardianPhone: getStoredPhoneLoginPhone(),
    })
    if (inviteToken && appStore.authenticated && appStore.role === 'parent') {
      void this.joinOrganizationByInvite(inviteToken)
    }
    if (inviteToken) {
      void this.loadClassInvite(inviteToken)
    }
    void this.loadApplicationData()
  },
  onShow() {
    const pendingInviteToken = decodeInviteToken(getPendingClassInviteToken())
    if (pendingInviteToken && pendingInviteToken !== this.data.inviteToken && appStore.authenticated && appStore.role === 'parent') {
      this.setData({ inviteToken: pendingInviteToken, isInvited: true })
      void this.joinOrganizationByInvite(pendingInviteToken)
      void this.loadClassInvite(pendingInviteToken)
    }
    void this.loadApplicationData()
  },
  async joinOrganizationByInvite(inviteToken: string) {
    if (!inviteToken || this.data.organizationSwitching) {
      return
    }
    this.setData({ organizationSwitching: true })
    try {
      const token = await switchParentOrganization({ invite_token: inviteToken })
      saveAuthToken(token)
      applicationLoadGuard.markDirty()
      await this.loadApplicationData()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '加入机构失败，请确认二维码有效')
    }
    finally {
      this.setData({ organizationSwitching: false })
    }
  },
  async loadApplicationData() {
    return applicationLoadGuard.run(() => this.loadApplicationDataInternal())
  },
  async loadApplicationDataInternal() {
    this.setData({ loading: true })
    try {
      const [me, applications] = await Promise.all([getParentMe(), getParentChildApplications()])
      const applicationViews = applications.items.map(toApplicationView)
      const existingPhone = this.data.guardianPhone.trim() || applicationViews[0]?.guardian_phone || getStoredPhoneLoginPhone()
      this.setData({
        hasBoundChild: (me.children || []).length > 0,
        applications: applicationViews,
        guardianPhone: existingPhone,
      })
    }
    catch (error) {
      if (isRequestError(error) && error.code === 'UNAUTHORIZED') {
        this.handleAuthExpired()
        applicationLoadGuard.markDirty()
        return
      }
      applicationLoadGuard.markDirty()
      this.showToast(error instanceof Error ? error.message : '申请信息加载失败，请重试')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  async loadClassInvite(token: string) {
    this.setData({ inviteLoading: true, inviteError: '' })
    try {
      const invite = await getClassInvite(token)
      this.setData({ invitedSchoolClassID: invite.school_class_id, inviteClassLabel: invite.label || `${invite.grade}${invite.class_name}`, inviteType: invite.school_class_id ? 'class' : 'organization', isInvited: true })
    }
    catch (error) {
      this.setData({ inviteError: error instanceof Error ? error.message : '班级邀请无效，请让老师重新生成二维码' })
    }
    finally {
      this.setData({ inviteLoading: false })
    }
  },
  handleInput(event: WechatMiniprogram.Input) {
    const field = event.currentTarget.dataset.field as ParentFormField
    this.setData({ [field]: event.detail.value })
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
    if (this.data.submitting) {
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
      this.showToast(this.data.inviteLoading ? '正在确认班级邀请，请稍候' : '班级邀请无效，请重新扫描二维码')
      return
    }
    const retainsExistingClass = Boolean(
      this.data.editingApplicationID
      && this.data.schoolName.trim() === this.data.editingOriginalSchoolName
      && this.data.classText.trim() === this.data.editingOriginalClassText,
    )
    const schoolClassID = this.data.invitedSchoolClassID || (retainsExistingClass ? this.data.editingSchoolClassID : 0)
    this.setData({ submitting: true })
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
      this.setData({
        editingApplicationID: 0,
        editingSchoolClassID: 0,
        editingOriginalSchoolName: '',
        editingOriginalClassText: '',
        childName: '',
        schoolName: '',
        classText: '',
        guardianName: '',
        guardianPhone,
        relationship: '',
        optionalVisible: false,
        applicationNotes: '',
        focusedField: '',
      })
      if (this.data.inviteToken) {
        clearPendingClassInviteToken()
      }
      applicationLoadGuard.markDirty()
      await this.loadApplicationData()
    }
    catch (error) {
      if (isRequestError(error) && error.code === 'UNAUTHORIZED') {
        this.handleAuthExpired()
        return
      }
      this.showToast(error instanceof Error ? error.message : '绑定申请提交失败')
    }
    finally {
      this.setData({ submitting: false })
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
      optionalVisible: true,
      applicationNotes: application.notes,
      editingOriginalSchoolName: application.school_name_input,
      editingOriginalClassText: [application.grade_input, application.class_name_input].filter(Boolean).join('') || application.grade || application.class_name,
    })
    this.showToast('已带入原申请，请补充资料后重新提交')
  },
  toggleOptionalInfo() {
    this.setData({ optionalVisible: !this.data.optionalVisible })
  },
  handleRefresh() {
    applicationLoadGuard.markDirty()
    void this.loadApplicationData()
  },
  handleEnterParent() {
    if (!this.data.hasBoundChild) {
      this.showToast('孩子绑定成功后才能进入家长端')
      return
    }
    if (typeof wx !== 'undefined') {
      wx.redirectTo({ url: '/pages/parent/index' })
    }
  },
  handleBackToIdentity() {
    if (typeof wx !== 'undefined') {
      wx.reLaunch({ url: '/pages/index/index' })
    }
  },
  handleAuthExpired() {
    appStore.clearAuthenticated()
    if (typeof wx !== 'undefined') {
      wx.reLaunch({ url: '/pages/index/index' })
    }
  },
  showToast(message: string) {
    showFeedback(this, message)
  },
})
