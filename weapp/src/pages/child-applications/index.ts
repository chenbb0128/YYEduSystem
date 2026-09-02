import type { ChildApplication } from '@/services/child-applications'
import type { SchoolClassRecord } from '@/services/master-data'
import { getStaffChildApplications, reviewChildApplication } from '@/services/child-applications'
import { getSchoolClasses } from '@/services/master-data'
import { getTeacherAssignments } from '@/services/teacher-assignments'
import { showFeedback } from '@/utils/feedback'

type ApplicationCard = ChildApplication & { status_label: string }

const statusLabels: Record<string, string> = {
  approved: '已通过',
  needs_info: '待补充资料',
  pending: '待老师审核',
  rejected: '未通过',
}

Page({
  data: {
    loading: false,
    applications: [] as ApplicationCard[],
    classOptions: [] as SchoolClassRecord[],
  },
  onLoad() {
    void this.loadApplications()
  },
  onShow() {
    void this.loadApplications()
  },
  async loadApplications() {
    this.setData({ loading: true })
    try {
      const [result, classes, assignments] = await Promise.all([getStaffChildApplications(), getSchoolClasses(), getTeacherAssignments()])
      const assignedClassIDs = new Set(assignments.items.filter(item => item.status === 'active').map(item => item.school_class_id))
      this.setData({
        applications: result.items.map((item) => {
          return {
            ...item,
            status_label: statusLabels[item.status] || item.status,
          }
        }),
        classOptions: classes.items.filter(item => item.status === 'active' && assignedClassIDs.has(item.id)),
      })
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '家长申请加载失败')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  async handleApprove(event: WechatMiniprogram.TouchEvent) {
    const applicationID = Number(event.currentTarget.dataset.applicationId)
    const application = this.data.applications.find(item => item.id === applicationID)
    if (!application) {
      return
    }
    let studentID: number | undefined
    if ((application.student_matches?.length || 0) > 1) {
      const selectedStudentID = await this.chooseStudentMatch(application.student_matches || [])
      if (!selectedStudentID) {
        return
      }
      studentID = selectedStudentID
    }
    const schoolClassID = application.school_class_id
    if (!schoolClassID) {
      const selectedClassID = await this.chooseClass(application)
      if (selectedClassID) {
        await this.runReview(applicationID, { status: 'approved', school_class_id: selectedClassID, student_id: studentID }, '已选择负责班级并绑定家长')
        return
      }
      if (application.school_id && this.data.classOptions.some(item => item.school_id === application.school_id)) {
        this.showToast('请先选择孩子所在的负责班级')
        return
      }
      if (application.school_id) {
        await this.confirmCreateClass(applicationID)
        return
      }
      this.showToast('学校还未被系统识别，请让管理员先维护学校资料')
      return
    }
    await this.runReview(applicationID, { status: 'approved', school_class_id: schoolClassID, student_id: studentID }, '已通过申请并自动绑定家长')
  },
  async confirmCreateClass(applicationID: number) {
    if (typeof wx === 'undefined') {
      return
    }
    wx.showModal({
      title: '确认建立班级',
      content: '系统会按家长填写的学校、年级和班级建立一个学校班级，并立即完成建档绑定。',
      confirmText: '建立并通过',
      success: (result) => {
        if (result.confirm) {
          void this.runReview(applicationID, { status: 'approved', create_school_class: true }, '已建立班级并通过申请')
        }
      },
    })
  },
  chooseStudentMatch(matches: NonNullable<ChildApplication['student_matches']>): Promise<number | null> {
    return new Promise((resolve) => {
      if (typeof wx === 'undefined' || !matches.length) {
        resolve(null)
        return
      }
      wx.showActionSheet({
        itemList: matches.map(item => `${item.name}${item.guardian_phone ? `（家长尾号${item.guardian_phone.slice(-4)}）` : ''}`),
        success: result => resolve(matches[result.tapIndex]?.id || null),
        fail: () => resolve(null),
      })
    })
  },
  chooseClass(application: ApplicationCard): Promise<number | null> {
    const options = application.school_id ? this.data.classOptions.filter(item => item.school_id === application.school_id) : []
    return new Promise((resolve) => {
      if (typeof wx === 'undefined' || !options.length) {
        resolve(null)
        return
      }
      wx.showActionSheet({
        itemList: options.map(item => `${item.grade}${item.name}`),
        success: result => resolve(options[result.tapIndex]?.id || null),
        fail: () => resolve(null),
      })
    })
  },
  async handleNeedsInfo(event: WechatMiniprogram.TouchEvent) {
    const applicationID = Number(event.currentTarget.dataset.applicationId)
    const note = await this.promptNote('请填写需要家长补充的内容')
    if (note === null) {
      return
    }
    await this.runReview(applicationID, { status: 'needs_info', review_note: note }, '已退回补充资料')
  },
  async handleReject(event: WechatMiniprogram.TouchEvent) {
    const applicationID = Number(event.currentTarget.dataset.applicationId)
    const note = await this.promptNote('请填写未通过原因')
    if (note === null) {
      return
    }
    await this.runReview(applicationID, { status: 'rejected', review_note: note }, '申请已拒绝')
  },
  promptNote(placeholderText: string): Promise<string | null> {
    return new Promise((resolve) => {
      if (typeof wx === 'undefined') {
        resolve(null)
        return
      }
      wx.showModal({
        title: '审核备注',
        editable: true,
        placeholderText,
        success: result => resolve(result.confirm ? (result.content || '').trim() : null),
        fail: () => resolve(null),
      })
    })
  },
  async runReview(applicationID: number, data: { status: 'approved' | 'needs_info' | 'rejected', school_class_id?: number, student_id?: number, create_school_class?: boolean, review_note?: string }, message: string) {
    this.setData({ loading: true })
    try {
      await reviewChildApplication(applicationID, data)
      this.showToast(message)
      await this.loadApplications()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '审核失败')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  showToast(message: string) {
    showFeedback(this, message)
  },
})
