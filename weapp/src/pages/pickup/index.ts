import type { PickupChangeRequest, PickupMemberStatus, PickupOperation, PickupOperationStudent } from '@/services/pickup'
import type { TeacherAssignmentRecord } from '@/services/teacher-assignments'
import { createTeacherLeaveRequest } from '@/services/leave'
import { addTemporaryPickupStudent, completeTemporaryPickupStudentProfile, confirmPickupOperation, correctPickupEvent, createPickupOperation, finishPickupOperation, getPickupChangeRequests, getPickupCloseCheck, getPickupEvents, getPickupHandoffs, getPickupHandoffTeachers, getPickupWorkbench, getToday, handoverPickupOperation, markPickupStudent, pickupOperationStatusLabel, pickupStatusLabel, reviewPickupChangeRequest, startPickupOperation, uploadPickupPhoto } from '@/services/pickup'
import { getTeacherAssignments } from '@/services/teacher-assignments'
import { showFeedback } from '@/utils/feedback'

interface OperationCard extends PickupOperation {
  students: Array<PickupOperationStudent & { checked_display: string, status_label: string }>
  status_label: string
  overdue: boolean
}

interface TeacherClassOption extends TeacherAssignmentRecord {
  label: string
}

interface PickupChangeRequestView extends PickupChangeRequest {
  requested_status_label: string
}

function isPickupOverdue(operation: PickupOperation, students: PickupOperationStudent[]) {
  if (operation.status !== 'started' || operation.operation_date !== getToday() || !operation.expected_pickup_time) {
    return false
  }
  if (!students.some(student => student.status === 'planned')) {
    return false
  }
  const expected = new Date(`${operation.operation_date}T${operation.expected_pickup_time}:00`)
  return !Number.isNaN(expected.getTime()) && Date.now() > expected.getTime() + 30 * 60 * 1000
}

Page({
  data: {
    date: getToday(),
    loading: false,
    operations: [] as OperationCard[],
    assignments: [] as TeacherClassOption[],
    shareClassID: 0,
    alerts: [] as Array<{ kind: string, message: string }>,
    changeRequests: [] as PickupChangeRequestView[],
  },
  onLoad() {
    void this.loadOperations()
  },
  onShow() {
    void this.loadOperations()
  },
  async loadOperations() {
    this.setData({ loading: true })
    try {
      const [result, assignmentResult, changeResult] = await Promise.all([
        getPickupWorkbench(this.data.date),
        getTeacherAssignments(),
        getPickupChangeRequests({ date: this.data.date, status: 'pending' }),
      ])
      const assignments = Array.from(
        new Map(
          assignmentResult.items
            .filter(item => item.status === 'active')
            .map(item => [item.school_class_id, {
              ...item,
              label: `学校 #${item.school_id} · ${item.grade}${item.class_name}`,
            }]),
        ).values(),
      )
      const operations: OperationCard[] = result.operations.map(entry => ({
        ...entry.operation,
        status_label: pickupOperationStatusLabel(entry.operation.status),
        overdue: isPickupOverdue(entry.operation, entry.students),
        students: entry.students.map(student => ({
          ...student,
          checked_display: student.status === 'planned' ? '等待现场确认' : (student.checked_at || ''),
          status_label: pickupStatusLabel(student.status),
        })),
      }))
      const overdueAlerts = operations.filter(item => item.overdue).map(item => ({ kind: 'pickup_overdue', operation_id: item.id, message: `${item.executing_teacher_name || item.teacher_name || '接送任务'}已超过预计出发时间，请确认现场进度` }))
      this.setData({ operations, assignments, alerts: [...result.alerts, ...overdueAlerts], changeRequests: changeResult.items.map(item => ({ ...item, requested_status_label: pickupStatusLabel(item.requested_status) })) })
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '接送任务加载失败')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  async handleCreateTodayTask() {
    const assignments = this.data.assignments
    if (!assignments.length) {
      this.showToast('暂未配置可操作的学校班级，请联系管理端设置授权')
      return
    }
    const assignment = await this.chooseTeacherClass(assignments)
    if (!assignment) {
      return
    }
    await this.runOperationAction(
      () => createPickupOperation({
        operation_date: this.data.date,
        school_class_id: assignment.school_class_id,
        pickup_mode: 'school_pickup',
      }),
      `${assignment.grade}${assignment.class_name}的今日任务已创建`,
    )
  },
  async handleReviewChangeRequest(event: WechatMiniprogram.TouchEvent) {
    const requestID = Number(event.currentTarget.dataset.requestId)
    const status = event.currentTarget.dataset.status as 'approved' | 'rejected'
    const item = this.data.changeRequests.find(request => request.id === requestID)
    if (!item || !requestID || (status !== 'approved' && status !== 'rejected')) {
      return
    }
    const submit = (reviewNote = '') => this.runOperationAction(
      () => reviewPickupChangeRequest(requestID, { status, review_note: reviewNote }),
      status === 'approved' ? '临时接送变更已同意，家长将收到通知' : '临时接送变更已拒绝，家长将收到通知',
    )
    if (status === 'approved' || typeof wx === 'undefined') {
      await submit()
      return
    }
    wx.showModal({
      title: '拒绝临时变更',
      editable: true,
      placeholderText: '请填写原因，便于家长理解',
      success: (result) => {
        if (result.confirm) {
          void submit(result.content?.trim() || '')
        }
      },
    })
  },
  chooseTeacherClass(assignments: TeacherClassOption[]): Promise<TeacherClassOption | null> {
    if (assignments.length === 1) {
      return Promise.resolve(assignments[0])
    }
    return new Promise((resolve) => {
      if (typeof wx === 'undefined') {
        resolve(null)
        return
      }
      wx.showActionSheet({
        itemList: assignments.map(item => item.label),
        success: result => resolve(assignments[result.tapIndex] || null),
        fail: () => resolve(null),
      })
    })
  },
  async handleShareParentInvite() {
    const assignment = await this.chooseTeacherClass(this.data.assignments)
    if (!assignment) {
      return
    }
    this.setData({ shareClassID: assignment.school_class_id })
    if (typeof wx === 'undefined') {
      this.showToast('请在微信中使用邀请分享')
      return
    }
    wx.showShareMenu({
      menus: ['shareAppMessage'],
      success: () => this.showToast('已准备邀请，请点击右上角分享给家长'),
      fail: error => this.showToast(error.errMsg || '邀请分享暂不可用'),
    })
  },
  onShareAppMessage() {
    const classID = this.data.shareClassID
    return {
      title: '邀请您添加孩子到托管班',
      path: classID ? `/pages/parent/index?schoolClassId=${classID}` : '/pages/parent/index',
    }
  },
  async handleStart(event: WechatMiniprogram.TouchEvent) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    await this.runOperationAction(() => startPickupOperation(operationId), '接送任务已开始')
  },
  async handleConfirm(event: WechatMiniprogram.TouchEvent) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    const confirm = (expectedPickupTime = '') => this.runOperationAction(() => confirmPickupOperation(operationId, { expected_pickup_time: expectedPickupTime }), '已确认今日接送，家长将收到老师安排')
    if (typeof wx === 'undefined') {
      await confirm()
      return
    }
    wx.showModal({
      title: '确认今日接送',
      editable: true,
      placeholderText: '可填写预计出发时间，例如 16:20',
      success: (result) => {
        if (result.confirm) {
          void confirm(result.content?.trim() || '')
        }
      },
    })
  },
  async handleFinish(event: WechatMiniprogram.TouchEvent) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    try {
      const check = await getPickupCloseCheck(operationId)
      if (!check.can_finish) {
        this.showToast(`还有 ${check.pending.length} 名学生未完成收班确认`)
        return
      }
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '收班检查失败')
      return
    }
    await this.runOperationAction(() => finishPickupOperation(operationId), '接送任务已完成')
  },
  handlePhotoCheckIn(event: WechatMiniprogram.TouchEvent) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    const studentId = Number(event.currentTarget.dataset.studentId)
    if (typeof wx === 'undefined') {
      return
    }
    wx.chooseMedia({
      count: 1,
      mediaType: ['image'],
      sourceType: ['camera', 'album'],
      success: (result) => {
        const filePath = result.tempFiles[0]?.tempFilePath
        if (filePath) {
          void this.uploadAndCheckIn(operationId, studentId, filePath)
        }
      },
      fail: error => this.showToast(error.errMsg || '未选择照片'),
    })
  },
  async uploadAndCheckIn(operationId: number, studentId: number, filePath: string) {
    if (this.data.loading) {
      return
    }
    this.setData({ loading: true })
    try {
      const photoURL = await uploadPickupPhoto(filePath, { operation_id: operationId })
      await markPickupStudent(operationId, studentId, 'picked_up', photoURL)
      this.showToast('已拍照并登记接到')
      await this.loadOperations()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '签到失败')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  async handleHandover(event: WechatMiniprogram.TouchEvent) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    const operation = this.data.operations.find(item => item.id === operationId)
    if (!operation || operation.status !== 'started' || this.data.loading || typeof wx === 'undefined') {
      return
    }
    try {
      const result = await getPickupHandoffTeachers(operationId)
      const teachers = result.items.filter(item => item.teacher_user_id !== operation.executing_teacher_user_id)
      if (!teachers.length) {
        this.showToast('当前班级暂时没有可交接的其他教师')
        return
      }
      wx.showActionSheet({
        itemList: teachers.map(item => item.teacher_name || item.username || '老师'),
        success: (choice) => {
          const teacher = teachers[choice.tapIndex]
          if (!teacher) {
            return
          }
          wx.showModal({
            title: `交接给${teacher.teacher_name || teacher.username || '老师'}`,
            editable: true,
            placeholderText: '可填写交接说明，例如：校门口临时交接',
            success: (modal) => {
              if (!modal.confirm) {
                return
              }
              void this.runOperationAction(
                () => handoverPickupOperation(operationId, { to_teacher_user_id: teacher.teacher_user_id, to_teacher_name: teacher.teacher_name, teacher_role: 'collaborator', note: modal.content?.trim() || '' }),
                '接送任务已完成交接，家长将收到老师变更通知',
              )
            },
          })
        },
      })
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '可交接教师加载失败')
    }
  },
  async handleViewHandoffs(event: WechatMiniprogram.TouchEvent) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    if (!operationId || typeof wx === 'undefined') {
      return
    }
    try {
      const result = await getPickupHandoffs(operationId)
      if (!result.items.length) {
        this.showToast('本次任务还没有交接记录')
        return
      }
      const latest = result.items[0]
      wx.showModal({
        title: '最近一次交接',
        content: `${latest.from_teacher_name || '原执行老师'} → ${latest.to_teacher_name || '接手老师'}\n${latest.handoff_at}${latest.note ? `\n${latest.note}` : ''}`,
        showCancel: false,
      })
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '交接记录加载失败')
    }
  },
  handleMark(event: WechatMiniprogram.TouchEvent) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    const studentId = Number(event.currentTarget.dataset.studentId)
    const status = event.currentTarget.dataset.status as Exclude<PickupMemberStatus, 'planned'>
    if (status === 'leave' && typeof wx !== 'undefined') {
      wx.showModal({
        title: '口头请假',
        editable: true,
        placeholderText: '请填写请假原因',
        success: (result) => {
          if (result.confirm && result.content?.trim()) {
            void this.createTeacherLeaveAndMark(operationId, studentId, result.content.trim())
          }
          else if (result.confirm) {
            this.showToast('请填写请假原因')
          }
        },
      })
      return
    }
    const requiresNote = status === 'absent' || status === 'parent_picked_up' || status === 'not_arrived' || status === 'left' || status === 'midway_left' || status === 'abnormal'
    if (requiresNote && typeof wx !== 'undefined') {
      wx.showModal({
        title: pickupStatusLabel(status),
        editable: true,
        placeholderText: '请填写接走人或异常说明',
        success: (result) => {
          if (result.confirm) {
            void this.markStudent(operationId, studentId, status, '', result.content || '')
          }
        },
      })
      return
    }
    void this.markStudent(operationId, studentId, status)
  },
  async createTeacherLeaveAndMark(operationId: number, studentId: number, reason: string) {
    const operation = this.data.operations.find(item => item.id === operationId)
    if (!operation) {
      this.showToast('接送任务已刷新，请重试')
      return
    }
    this.setData({ loading: true })
    try {
      await createTeacherLeaveRequest({ student_id: studentId, leave_date: operation.operation_date, reason })
      await markPickupStudent(operationId, studentId, 'leave', '', reason)
      this.showToast('已登记口头请假')
      await this.loadOperations()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '口头请假登记失败')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  async markStudent(operationId: number, studentId: number, status: Exclude<PickupMemberStatus, 'planned'>, photoUrl = '', note = '') {
    if (this.data.loading) {
      return
    }
    this.setData({ loading: true })
    try {
      await markPickupStudent(operationId, studentId, status, photoUrl, note)
      this.showToast(`已登记：${pickupStatusLabel(status)}`)
      await this.loadOperations()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '登记失败')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  async handleCorrectStatus(event: WechatMiniprogram.TouchEvent) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    const studentId = Number(event.currentTarget.dataset.studentId)
    if (!operationId || !studentId || this.data.loading || typeof wx === 'undefined') {
      return
    }
    try {
      const events = await getPickupEvents(operationId)
      const original = events.items.find(item => item.student_id === studentId && item.event_type !== 'correction')
      if (!original) {
        this.showToast('暂时找不到可更正的原始记录')
        return
      }
      const statuses: Array<Exclude<PickupMemberStatus, 'planned'>> = ['picked_up', 'self_arrived', 'parent_picked_up', 'leave', 'absent', 'arrived', 'not_arrived', 'left', 'midway_left', 'abnormal']
      const labels = statuses.map(status => pickupStatusLabel(status))
      wx.showActionSheet({
        itemList: labels,
        success: (choice) => {
          const status = statuses[choice.tapIndex]
          if (!status) {
            return
          }
          wx.showModal({
            title: `更正为${pickupStatusLabel(status)}`,
            editable: true,
            placeholderText: '请填写更正原因，便于家长和机构核对',
            success: (result) => {
              if (!result.confirm || !result.content?.trim()) {
                if (result.confirm) {
                  this.showToast('请填写更正原因')
                }
                return
              }
              void this.runOperationAction(() => correctPickupEvent(operationId, original.id, status, result.content.trim()), '接送记录已更正')
            },
          })
        },
      })
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '读取接送记录失败')
    }
  },
  async handleAddTemporaryStudent(event: WechatMiniprogram.TouchEvent) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    if (typeof wx === 'undefined') {
      return
    }
    wx.showModal({
      title: '新增今日临时学生',
      editable: true,
      placeholderText: '请输入学生姓名，详细档案稍后补充',
      success: (result) => {
        if (!result.confirm || !result.content?.trim()) {
          return
        }
        void this.runOperationAction(() => addTemporaryPickupStudent(operationId, { name: result.content.trim(), note: '教师现场临时添加，档案待补充' }), '临时学生已加入今日名单')
      },
    })
  },
  async handleCompleteProfile(event: WechatMiniprogram.TouchEvent) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    const studentId = Number(event.currentTarget.dataset.studentId)
    if (typeof wx === 'undefined') {
      return
    }
    const fields = [
      { key: 'guardian_phone', title: '家长手机号', placeholderText: '请输入家长手机号' },
      { key: 'gender', title: '性别', placeholderText: '请输入 unknown、male 或 female' },
      { key: 'student_no', title: '学号', placeholderText: '可留空' },
      { key: 'emergency_contact', title: '紧急联系人', placeholderText: '可留空' },
      { key: 'emergency_phone', title: '紧急联系电话', placeholderText: '可留空' },
      { key: 'notes', title: '档案备注', placeholderText: '可留空' },
    ] as const
    const profile: Record<string, string> = {}
    for (const field of fields) {
      const value = await this.promptProfileField(field.title, field.placeholderText)
      if (value === null) {
        return
      }
      profile[field.key] = value
    }
    await this.runOperationAction(() => completeTemporaryPickupStudentProfile(operationId, studentId, profile), '临时学生档案已补充')
  },
  promptProfileField(title: string, placeholderText: string): Promise<string | null> {
    return new Promise((resolve) => {
      if (typeof wx === 'undefined') {
        resolve(null)
        return
      }
      wx.showModal({
        title,
        editable: true,
        placeholderText,
        success: result => resolve(result.confirm ? (result.content || '').trim() : null),
        fail: () => resolve(null),
      })
    })
  },
  async runOperationAction<T>(action: () => Promise<T>, message: string) {
    if (this.data.loading) {
      return
    }
    this.setData({ loading: true })
    try {
      await action()
      this.showToast(message)
      await this.loadOperations()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '操作失败')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  showToast(message: string) {
    showFeedback(this, message)
  },
  pickupStatusLabel,
  pickupOperationStatusLabel,
})
