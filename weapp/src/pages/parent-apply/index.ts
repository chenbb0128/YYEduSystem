import type { ChildApplication } from '@/services/child-applications'
import { getStoredPhoneLoginPhone } from '@/services/auth'
import { createParentChildApplication, getParentChildApplications, updateParentChildApplication } from '@/services/child-applications'
import { getParentMe } from '@/services/parent'
import { isRequestError } from '@/services/request'
import { useAppStore } from '@/stores'
import { showFeedback } from '@/utils/feedback'

const appStore = useAppStore()

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

Page({
  data: {
    loading: false,
    hasBoundChild: false,
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
  },
  onLoad(options: Record<string, string | undefined> = {}) {
    const invitedSchoolClassID = Number(options.schoolClassId || 0)
    this.setData({
      invitedSchoolClassID: Number.isFinite(invitedSchoolClassID) ? invitedSchoolClassID : 0,
      guardianPhone: getStoredPhoneLoginPhone(),
    })
    void this.loadApplicationData()
  },
  onShow() {
    void this.loadApplicationData()
  },
  async loadApplicationData() {
    this.setData({ loading: true })
    try {
      const [me, applications] = await Promise.all([getParentMe(), getParentChildApplications()])
      this.setData({
        hasBoundChild: (me.children || []).length > 0,
        applications: applications.items.map(toApplicationView),
      })
    }
    catch (error) {
      if (isRequestError(error) && error.code === 'UNAUTHORIZED') {
        this.handleAuthExpired()
        return
      }
      this.showToast(error instanceof Error ? error.message : '申请信息加载失败，请重试')
    }
    finally {
      this.setData({ loading: false })
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
      this.setData({
        editingApplicationID: 0,
        editingSchoolClassID: 0,
        editingOriginalSchoolName: '',
        editingOriginalClassText: '',
        childName: '',
        schoolName: '',
        classText: '',
        guardianName: '',
        guardianPhone: getStoredPhoneLoginPhone(),
        relationship: '',
        applicationNotes: '',
        focusedField: '',
      })
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
  handleRefresh() {
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
