import type { ChildApplication } from '@/services/child-applications'
import type { SchoolClassRecord } from '@/services/master-data'
import { getStaffChildApplications, reviewChildApplication } from '@/services/child-applications'
import { getSchoolClasses, getSchools } from '@/services/master-data'
import { isRequestError } from '@/services/request'
import { getTeacherAssignments } from '@/services/teacher-assignments'
import { showFeedback } from '@/utils/feedback'

type ClassOption = SchoolClassRecord & { school_name: string }
type ApplicationCard = ChildApplication & { approve_action_label: string, class_auto_note: string, status_label: string, system_label: string }

const statusLabels: Record<string, string> = {
  approved: '已通过',
  needs_info: '待补充资料',
  pending: '待老师审核',
  rejected: '未通过',
}

function compactClassLabel(grade: string, className: string) {
  return [grade, className].filter(Boolean).join('')
}

function applicationClassLabel(application: ChildApplication) {
  return compactClassLabel(application.grade || application.grade_input, application.class_name || application.class_name_input) || '班级待确认'
}

function buildSystemLabel(application: ChildApplication, classOptions: ClassOption[], schoolNames: Map<number, string>) {
  if (application.school_class_id) {
    const matchedClass = classOptions.find(item => item.id === application.school_class_id)
    const schoolName = matchedClass ? matchedClass.school_name : (application.school_id ? schoolNames.get(application.school_id) : '')
    return `${schoolName || application.school_name_input || '学校待确认'} · ${matchedClass ? compactClassLabel(matchedClass.grade, matchedClass.name) : applicationClassLabel(application)} · 已匹配班级`
  }
  if (application.school_id) {
    const schoolName = schoolNames.get(application.school_id) || application.school_name_input || '学校待确认'
    return `${schoolName} · ${applicationClassLabel(application)} · 班级待建档`
  }
  if (application.school_name_input) {
    return `${application.school_name_input} · ${applicationClassLabel(application)} · 学校待建档`
  }
  return `学校待确认 · ${applicationClassLabel(application)}`
}

function buildAutoNote(application: ChildApplication) {
  if (application.school_class_id) {
    return ''
  }
  if (application.school_id) {
    return '已识别学校，但没有匹配到同名班级；点击“建班并通过”后会自动建立班级并绑定家长。'
  }
  if (application.school_name_input) {
    return '未匹配到已维护学校；管理员可点击“建档并通过”自动建立学校和班级，普通老师请联系管理员先维护学校资料。'
  }
  return '家长未填写学校信息，请先让家长补充资料。'
}

function canCreateApplicationClass(application: ChildApplication) {
  return Boolean(application.school_name_input?.trim() && (application.grade || application.grade_input || application.class_name || application.class_name_input))
}

function buildApproveActionLabel(application: ChildApplication) {
  if (application.school_class_id) {
    return '通过审核'
  }
  if (canCreateApplicationClass(application)) {
    return application.school_id ? '建班并通过' : '建档并通过'
  }
  return '通过审核'
}

Page({
  data: {
    loading: false,
    applications: [] as ApplicationCard[],
    classOptions: [] as ClassOption[],
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
      const [result, schools, classes, assignments] = await Promise.all([getStaffChildApplications(), getSchools(), getSchoolClasses(), getTeacherAssignments()])
      const schoolNames = new Map(schools.items.map(item => [item.id, item.name]))
      const assignedClassIDs = new Set(assignments.items.filter(item => item.status === 'active').map(item => item.school_class_id))
      const activeClasses = classes.items.filter(item => item.status === 'active')
      const classOptions = (assignedClassIDs.size ? activeClasses.filter(item => assignedClassIDs.has(item.id)) : activeClasses).map(item => ({
        ...item,
        school_name: schoolNames.get(item.school_id) || `学校 #${item.school_id}`,
      }))
      this.setData({
        applications: result.items.map((item) => {
          return {
            ...item,
            approve_action_label: buildApproveActionLabel(item),
            class_auto_note: buildAutoNote(item),
            status_label: statusLabels[item.status] || item.status,
            system_label: buildSystemLabel(item, classOptions, schoolNames),
          }
        }),
        classOptions,
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
      if (canCreateApplicationClass(application)) {
        await this.confirmCreateClass(application)
        return
      }
      const selectedClassID = await this.chooseClass(application)
      if (selectedClassID) {
        await this.runReview(applicationID, { status: 'approved', school_class_id: selectedClassID, student_id: studentID }, '已选择负责班级并绑定家长')
        return
      }
      this.showToast('请先让家长补充学校和班级信息')
      return
    }
    await this.runReview(applicationID, { status: 'approved', school_class_id: schoolClassID, student_id: studentID }, '已通过申请并自动绑定家长')
  },
  async confirmCreateClass(application: ApplicationCard) {
    if (typeof wx === 'undefined') {
      return
    }
    const content = application.school_id
      ? `系统会按「${application.school_name_input || '已识别学校'} · ${applicationClassLabel(application)}」建立学校班级，并立即完成建档绑定。`
      : `系统会按家长填写的「${application.school_name_input} · ${applicationClassLabel(application)}」建立学校和班级，并立即完成建档绑定。若当前账号不是管理员，请改用管理员账号审核。`
    wx.showModal({
      title: application.school_id ? '确认建立班级' : '确认建立学校和班级',
      content,
      confirmText: '建立并通过',
      success: (result) => {
        if (result.confirm) {
          void this.runReview(application.id, { status: 'approved', create_school_class: true }, application.school_id ? '已建立班级并通过申请' : '已建立学校和班级并通过申请')
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
    const options = application.school_id ? this.data.classOptions.filter(item => item.school_id === application.school_id) : this.data.classOptions
    return new Promise((resolve) => {
      if (typeof wx === 'undefined' || !options.length) {
        resolve(null)
        return
      }
      wx.showActionSheet({
        itemList: options.map(item => `${item.school_name} · ${item.grade}${item.name}`),
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
      if (data.status === 'approved' && data.create_school_class && isRequestError(error) && error.status === 403) {
        this.showToast('当前账号不能新建学校，请用管理员账号审核，或先在档案中心维护学校和班级')
        return
      }
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
