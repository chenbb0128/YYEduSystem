import type { TeacherLeaveRequest } from '@/services/leave'
import type { PickupChangeRequest, PickupMemberStatus, PickupOperation, PickupOperationStudent } from '@/services/pickup'
import type { TeacherAssignmentRecord } from '@/services/teacher-assignments'
import { createTeacherLeaveRequest, getTeacherLeaveRequests, reviewTeacherLeaveRequest } from '@/services/leave'
import { addTemporaryPickupStudent, bulkArrivePickupStudents, completeTemporaryPickupStudentProfile, confirmPickupOperation, correctPickupEvent, createBatchNotification, createPickupOperation, finishPickupOperation, getPickupChangeRequests, getPickupCloseCheck, getPickupEvents, getPickupHandoffs, getPickupHandoffTeachers, getPickupWorkbench, getToday, handoverPickupOperation, markPickupStudent, pickupOperationStatusLabel, pickupStatusLabel, reviewPickupChangeRequest, startPickupOperation, uploadPickupPhoto } from '@/services/pickup'
import { getDailyExceptions } from '@/services/report'
import { getTeacherAssignments } from '@/services/teacher-assignments'
import { showFeedback } from '@/utils/feedback'

interface OperationCard extends PickupOperation {
  students: Array<PickupOperationStudent & { checked_display: string, status_label: string, photo_status: string, can_bulk_arrive: boolean, selected_for_bulk: boolean }>
  class_label: string
  counts: Record<string, number>
  pending_photo_count: number
  bulk_arrival_count: number
  selected_arrival_count: number
  all_arrival_selected: boolean
  status_label: string
  overdue: boolean
}

interface TeacherClassOption extends TeacherAssignmentRecord {
  label: string
}

interface PickupChangeRequestView extends PickupChangeRequest {
  requested_status_label: string
}

interface TeacherLeaveRequestView extends TeacherLeaveRequest {
  status_label: string
}

interface WorkbenchTotals {
  tasks: number
  students: number
  planned: number
  picked_up: number
  arrived: number
  finished: number
  abnormal: number
  pending_photo: number
  profile_pending: number
  [key: string]: number
}

interface WorkbenchTodo {
  key: string
  kind: string
  title: string
  message: string
}

function formatCheckedAt(value?: string) {
  if (!value) {
    return ''
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return `已记录 ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
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

function uniqueMessages(messages: string[]) {
  return Array.from(new Set(messages.map(message => message.trim()).filter(Boolean)))
}

let operationsLoadPromise: Promise<void> | undefined

Page({
  data: {
    date: getToday(),
    loading: false,
    operations: [] as OperationCard[],
    assignments: [] as TeacherClassOption[],
    shareClassID: 0,
    alerts: [] as Array<{ kind: string, message: string }>,
    changeRequests: [] as PickupChangeRequestView[],
    leaveRequests: [] as TeacherLeaveRequestView[],
    totals: {
      tasks: 0,
      students: 0,
      planned: 0,
      picked_up: 0,
      arrived: 0,
      finished: 0,
      abnormal: 0,
      pending_photo: 0,
      profile_pending: 0,
    } as WorkbenchTotals,
    handledStudentCount: 0,
    overallProgress: 0,
    overallProgressStyle: 'width: 0%;',
    todoItems: [] as WorkbenchTodo[],
    selectedArrivalKeys: [] as string[],
    closeCheckOperationId: 0,
  },
  onLoad() {
    void this.loadOperations()
  },
  onShow() {
    void this.loadOperations()
  },
  async onPullDownRefresh() {
    try {
      await this.loadOperations()
    }
    finally {
      if (typeof wx !== 'undefined') {
        wx.stopPullDownRefresh()
      }
    }
  },
  async loadOperations() {
    // onLoad and onShow can fire back-to-back when a page is first opened.
    // Share the in-flight request so the newer response cannot overwrite the
    // selection state with a stale duplicate response.
    if (operationsLoadPromise) {
      return operationsLoadPromise
    }
    operationsLoadPromise = this.loadOperationsInternal()
    try {
      await operationsLoadPromise
    }
    finally {
      operationsLoadPromise = undefined
    }
  },
  async loadOperationsInternal() {
    this.setData({ loading: true })
    try {
      const [workbenchResult, assignmentResult, changeResult, leaveResult] = await Promise.allSettled([
        getPickupWorkbench(this.data.date),
        getTeacherAssignments(),
        getPickupChangeRequests({ date: this.data.date, status: 'pending' }),
        getTeacherLeaveRequests(this.data.date),
      ])
      if (workbenchResult.status === 'rejected') {
        throw workbenchResult.reason
      }
      const result = workbenchResult.value
      const assignmentItems = assignmentResult.status === 'fulfilled' ? assignmentResult.value.items : []
      const changeItems = changeResult.status === 'fulfilled' ? changeResult.value.items : []
      const leaveItems = leaveResult.status === 'fulfilled' ? leaveResult.value.items : []
      const assignments = Array.from(
        new Map(
          assignmentItems
            .filter(item => item.status === 'active')
            .map(item => [item.school_class_id, {
              ...item,
              label: `学校 #${item.school_id} · ${item.grade}${item.class_name}`,
            }]),
        ).values(),
      )
      const selectedKeys = new Set(this.data.selectedArrivalKeys)
      const classLabels = new Map(assignments.map(item => [item.school_class_id, item.label]))
      const operations: OperationCard[] = result.operations.map((entry) => {
        const counts = entry.counts || {}
        const pendingPhotoCount = entry.students.filter(student => student.status === 'picked_up' && !student.photo_url).length
        const eligibleStudents = entry.students.filter(student => student.status === 'picked_up' || student.status === 'self_arrived')
        const selectedStudents = eligibleStudents.filter(student => selectedKeys.has(`${entry.operation.id}:${student.student_id}`))
        return {
          ...entry.operation,
          class_label: classLabels.get(entry.operation.school_class_id) || `学校班级 #${entry.operation.school_class_id}`,
          counts,
          pending_photo_count: pendingPhotoCount,
          bulk_arrival_count: eligibleStudents.length,
          selected_arrival_count: selectedStudents.length,
          all_arrival_selected: eligibleStudents.length > 0 && selectedStudents.length === eligibleStudents.length,
          status_label: pickupOperationStatusLabel(entry.operation.status),
          overdue: isPickupOverdue(entry.operation, entry.students),
          students: entry.students.map(student => ({
            ...student,
            checked_display: student.status === 'planned' ? '等待现场确认' : formatCheckedAt(student.checked_at),
            photo_status: student.status === 'picked_up' ? (student.photo_url ? '照片已上传' : '照片待补') : '',
            can_bulk_arrive: entry.operation.status === 'started' && (student.status === 'picked_up' || student.status === 'self_arrived'),
            selected_for_bulk: entry.operation.status === 'started' && (student.status === 'picked_up' || student.status === 'self_arrived') && selectedKeys.has(`${entry.operation.id}:${student.student_id}`),
            status_label: pickupStatusLabel(student.status),
          })),
        }
      })
      const overdueAlerts = operations.filter(item => item.overdue).map(item => ({ kind: 'pickup_overdue', operation_id: item.id, message: `${item.executing_teacher_name || item.teacher_name || '接送任务'}已超过预计出发时间，请确认现场进度` }))
      const totals = {
        tasks: result.totals?.tasks || 0,
        students: result.totals?.students || 0,
        planned: result.totals?.planned || 0,
        picked_up: result.totals?.picked_up || 0,
        arrived: result.totals?.arrived || 0,
        finished: result.totals?.finished || 0,
        abnormal: result.totals?.abnormal || 0,
        pending_photo: result.totals?.pending_photo || 0,
        profile_pending: result.totals?.profile_pending || 0,
      }
      const handledStudentCount = Math.max(0, totals.students - totals.planned)
      const overallProgress = totals.students ? Math.round((handledStudentCount / totals.students) * 100) : 0
      const todoItems: WorkbenchTodo[] = []
      for (const operation of operations) {
        if (operation.status === 'draft') {
          todoItems.push({ key: `confirm-${operation.id}`, kind: 'confirm', title: '确认今日任务', message: `${operation.class_label}还没有确认接送名单` })
        }
        else if (operation.status === 'confirmed') {
          todoItems.push({ key: `start-${operation.id}`, kind: 'start', title: '确认出发', message: `${operation.class_label}已确认名单，出发前请点击确认出发` })
        }
        if (operation.status === 'started' && (operation.counts.planned || 0) > 0) {
          todoItems.push({ key: `pickup-${operation.id}`, kind: 'pickup', title: '完成校门口点名', message: `${operation.class_label}还有 ${operation.counts.planned} 名学生待现场确认` })
        }
        if (operation.status === 'started' && (operation.counts.picked_up || 0) > 0) {
          todoItems.push({ key: `arrive-${operation.id}`, kind: 'arrive', title: '完成到班确认', message: `${operation.class_label}有 ${operation.counts.picked_up} 名学生待批量确认到班` })
        }
        if (operation.pending_photo_count > 0) {
          todoItems.push({ key: `photo-${operation.id}`, kind: 'photo', title: '补传接送照片', message: `${operation.class_label}有 ${operation.pending_photo_count} 张接送照片待补` })
        }
        if (operation.counts.not_arrived || operation.counts.abnormal || operation.counts.absent) {
          const count = (operation.counts.not_arrived || 0) + (operation.counts.abnormal || 0) + (operation.counts.absent || 0)
          todoItems.push({ key: `exception-${operation.id}`, kind: 'exception', title: '处理异常', message: `${operation.class_label}有 ${count} 名学生需要收班核对` })
        }
        if (operation.students.some(student => student.profile_pending)) {
          todoItems.push({ key: `profile-${operation.id}`, kind: 'profile', title: '补充临时档案', message: `${operation.class_label}有临时学生档案待补充` })
        }
      }
      const availableSelectionKeys = new Set(operations.flatMap(operation => operation.students.filter(student => student.can_bulk_arrive).map(student => `${operation.id}:${student.student_id}`)))
      const selectedArrivalKeys = this.data.selectedArrivalKeys.filter(key => availableSelectionKeys.has(key))
      const loadAlerts = [...result.alerts, ...overdueAlerts]
      if (assignmentResult.status === 'rejected') {
        loadAlerts.push({ kind: 'support', operation_id: 0, message: '教师班级授权加载失败，暂时无法新建接送任务；已有任务仍可继续操作' })
      }
      if (changeResult.status === 'rejected') {
        loadAlerts.push({ kind: 'support', operation_id: 0, message: '临时接送申请加载失败，请稍后刷新查看' })
      }
      if (leaveResult.status === 'rejected') {
        loadAlerts.push({ kind: 'support', operation_id: 0, message: '请假申请加载失败，请稍后刷新查看' })
      }
      this.setData({ operations, assignments, totals, handledStudentCount, overallProgress, overallProgressStyle: `width: ${overallProgress}%;`, todoItems, selectedArrivalKeys, alerts: loadAlerts, changeRequests: changeItems.map(item => ({ ...item, requested_status_label: pickupStatusLabel(item.requested_status) })), leaveRequests: leaveItems.map(item => ({ ...item, status_label: item.status === 'pending' ? '待处理' : item.status === 'approved' ? '已同意' : item.status === 'rejected' ? '已拒绝' : '已取消' })) })
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
  async handleReviewLeaveRequest(event: WechatMiniprogram.TouchEvent) {
    const requestID = Number(event.currentTarget.dataset.requestId)
    const status = event.currentTarget.dataset.status as 'approved' | 'rejected'
    const item = this.data.leaveRequests.find(request => request.id === requestID)
    if (!item || !requestID || (status !== 'approved' && status !== 'rejected')) {
      return
    }
    const submit = (teacherNote = '') => this.runOperationAction(
      () => reviewTeacherLeaveRequest(requestID, { status, teacher_note: teacherNote }),
      status === 'approved' ? '请假已同意，家长将收到通知' : '请假已拒绝，家长将收到通知',
    )
    if (status === 'approved' || typeof wx === 'undefined') {
      await submit()
      return
    }
    wx.showModal({
      title: '拒绝请假申请',
      editable: true,
      placeholderText: '可填写原因，便于家长理解',
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
      this.showToast('请在微信中打开班级邀请二维码')
      return
    }
    wx.navigateTo({
      url: `/pages/class-invite/index?schoolClassId=${assignment.school_class_id}&classLabel=${encodeURIComponent(`${assignment.grade}${assignment.class_name}`)}`,
      fail: error => this.showToast(error.errMsg || '邀请二维码页面打开失败'),
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
    if (!operationId || this.data.loading || this.data.closeCheckOperationId === operationId) {
      return
    }
    const operation = this.data.operations.find(item => item.id === operationId)
    if (!operation) {
      this.showToast('接送任务已刷新，请重试')
      return
    }
    this.setData({ closeCheckOperationId: operationId })
    let modalOpen = false
    try {
      const check = await getPickupCloseCheck(operationId)
      let dailyExceptionMessages: string[] = []
      try {
        const dailyExceptions = await getDailyExceptions(this.data.date, false, { school_class_id: operation.school_class_id, operation_id: operationId })
        dailyExceptionMessages = dailyExceptions.items
          .filter(item => item.category !== 'pickup' || item.operation_id === operationId)
          .slice(0, 4)
          .map(item => item.message)
      }
      catch {
        // The detailed cross-module checklist is helpful but must not prevent
        // the pickup state transition when the reporting endpoint is down.
      }
      if (!check.can_finish) {
        const reasons = uniqueMessages((check.blockers || []).map(item => item.message))
        if (!reasons.length && check.pending.length) {
          reasons.push(`还有 ${check.pending.length} 名学生未完成接送状态确认`)
        }
        if (!reasons.length && check.pending_photo_count) {
          reasons.push(`还有 ${check.pending_photo_count} 张接送照片待补`)
        }
        this.showCloseCheckModal('暂不能结束接送', reasons.length ? reasons : ['当前任务还未满足结束条件'], false)
        modalOpen = true
        return
      }
      const warnings = uniqueMessages((check.warnings || []).map(item => item.message))
      if (!check.warnings && check.exceptions.length) {
        warnings.push(...check.exceptions.map(item => item.message))
      }
      if (!warnings.length && check.profile_pending_count) {
        warnings.push(`还有 ${check.profile_pending_count} 名临时学生档案待补`)
      }
      warnings.push(...dailyExceptionMessages)
      this.showCloseCheckModal('收班检查', warnings.length ? warnings.slice(0, 6) : ['接送状态已全部登记，可以结束今天的接送任务'], true, operationId)
      modalOpen = true
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '收班检查失败')
    }
    finally {
      if (!modalOpen && this.data.closeCheckOperationId === operationId) {
        this.setData({ closeCheckOperationId: 0 })
      }
    }
  },
  showCloseCheckModal(title: string, lines: string[], canConfirm: boolean, operationId = 0) {
    if (typeof wx === 'undefined') {
      this.setData({ closeCheckOperationId: 0 })
      if (canConfirm && operationId) {
        void this.runOperationAction(() => finishPickupOperation(operationId), '接送任务已完成')
      }
      else {
        this.showToast(lines[0] || '收班检查未通过')
      }
      return
    }
    wx.showModal({
      title,
      content: lines.map((line, index) => `${index + 1}. ${line}`).join('\n'),
      confirmText: canConfirm ? '确认结束' : '知道了',
      showCancel: canConfirm,
      success: (result) => {
        if (canConfirm && result.confirm && operationId) {
          void this.runOperationAction(() => finishPickupOperation(operationId), '接送任务已完成')
        }
      },
      fail: (error) => {
        this.setData({ closeCheckOperationId: 0 })
        this.showToast(error.errMsg || '收班检查弹窗打开失败')
      },
      complete: () => {
        this.setData({ closeCheckOperationId: 0 })
      },
    })
  },
  handlePhotoCheckIn(event: WechatMiniprogram.TouchEvent) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    const studentId = Number(event.currentTarget.dataset.studentId)
    this.chooseStudentPhoto(operationId, studentId)
  },
  chooseStudentPhoto(operationId: number, studentId: number) {
    if (!operationId || !studentId || this.data.loading) {
      return
    }
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
  handleBulkSelectionChange(event: WechatMiniprogram.CheckboxGroupChange) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    if (!operationId) {
      return
    }
    const values = ((event.detail as unknown as { value?: string[] }).value || []).filter(Boolean)
    const prefix = `${operationId}:`
    const selectedArrivalKeys = [
      ...this.data.selectedArrivalKeys.filter(key => !key.startsWith(prefix)),
      ...values.map(value => `${prefix}${Number(value)}`).filter(key => !key.endsWith(':0')),
    ]
    this.setData({ selectedArrivalKeys })
    this.applyBulkSelection(selectedArrivalKeys)
  },
  handleSelectAllArrive(event: WechatMiniprogram.TouchEvent) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    const operation = this.data.operations.find(item => item.id === operationId)
    if (!operation || !operation.bulk_arrival_count) {
      return
    }
    const prefix = `${operationId}:`
    const selected = new Set(this.data.selectedArrivalKeys)
    if (operation.all_arrival_selected) {
      for (const student of operation.students) {
        selected.delete(`${prefix}${student.student_id}`)
      }
    }
    else {
      for (const student of operation.students) {
        if (student.can_bulk_arrive) {
          selected.add(`${prefix}${student.student_id}`)
        }
      }
    }
    const selectedArrivalKeys = [...selected]
    this.setData({ selectedArrivalKeys })
    this.applyBulkSelection(selectedArrivalKeys)
  },
  async handleBulkArrive(event: WechatMiniprogram.TouchEvent) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    const operation = this.data.operations.find(item => item.id === operationId)
    if (!operation || this.data.loading) {
      return
    }
    const prefix = `${operationId}:`
    const selectedKeys = new Set(this.data.selectedArrivalKeys)
    const studentIds = operation.students
      .filter(student => student.can_bulk_arrive && selectedKeys.has(`${prefix}${student.student_id}`))
      .map(student => student.student_id)
    if (!studentIds.length) {
      this.showToast('请先选择需要确认到班的学生')
      return
    }
    const submit = () => this.runOperationAction(
      async () => {
        await bulkArrivePickupStudents(operationId, studentIds)
        this.setData({ selectedArrivalKeys: this.data.selectedArrivalKeys.filter(key => !key.startsWith(prefix)) })
      },
      `已确认 ${studentIds.length} 名学生到班`,
    )
    if (typeof wx === 'undefined') {
      await submit()
      return
    }
    wx.showModal({
      title: '批量确认到班',
      content: `确认已选择的 ${studentIds.length} 名学生都已安全到达托管班吗？`,
      success: (result) => {
        if (result.confirm) {
          void submit()
        }
      },
    })
  },
  handleBatchNotify(event: WechatMiniprogram.TouchEvent) {
    this.openBatchNotify(Number(event.currentTarget.dataset.operationId))
  },
  openBatchNotify(operationId: number) {
    const operation = this.data.operations.find(item => item.id === operationId)
    const studentIds = operation?.students.filter(student => !student.is_temporary && student.status !== 'leave').map(student => student.student_id) || []
    if (!operation || !studentIds.length || typeof wx === 'undefined') {
      this.showToast('当前班级没有可通知的正式学生')
      return
    }
    wx.showModal({
      title: '通知本班家长',
      editable: true,
      placeholderText: '例如：今天作业较多，请提醒孩子完成后再休息',
      success: (result) => {
        const content = result.content?.trim() || ''
        if (!result.confirm) {
          return
        }
        if (!content) {
          this.showToast('请输入通知内容')
          return
        }
        void this.runOperationAction(
          () => createBatchNotification({ student_ids: studentIds, title: '老师通知', content }),
          `已通知 ${studentIds.length} 位家长`,
        )
      },
    })
  },
  applyBulkSelection(selectedArrivalKeys: string[]) {
    const selected = new Set(selectedArrivalKeys)
    const operations = this.data.operations.map((operation) => {
      const prefix = `${operation.id}:`
      const selectedCount = operation.students.filter(student => student.can_bulk_arrive && selected.has(`${prefix}${student.student_id}`)).length
      return {
        ...operation,
        selected_arrival_count: selectedCount,
        all_arrival_selected: operation.bulk_arrival_count > 0 && selectedCount === operation.bulk_arrival_count,
        students: operation.students.map(student => ({
          ...student,
          selected_for_bulk: student.can_bulk_arrive && selected.has(`${prefix}${student.student_id}`),
        })),
      }
    })
    this.setData({ operations })
  },
  async handleHandover(event: WechatMiniprogram.TouchEvent) {
    await this.openHandover(Number(event.currentTarget.dataset.operationId))
  },
  async openHandover(operationId: number) {
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
    await this.openViewHandoffs(Number(event.currentTarget.dataset.operationId))
  },
  async openViewHandoffs(operationId: number) {
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
  handleOpenExceptions() {
    if (typeof wx !== 'undefined') {
      wx.navigateTo({ url: '/pages/exceptions/index' })
    }
  },
  handleTaskMoreActions(event: WechatMiniprogram.TouchEvent) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    const operation = this.data.operations.find(item => item.id === operationId)
    if (!operation || (operation.status !== 'confirmed' && operation.status !== 'started') || this.data.loading || typeof wx === 'undefined') {
      return
    }
    const actions = operation.status === 'confirmed'
      ? [{ label: '临时加学生', action: 'temporary' as const }]
      : [
          { label: '临时加学生', action: 'temporary' as const },
          { label: '通知本班家长', action: 'notify' as const },
          { label: '途中交接', action: 'handover' as const },
          { label: '查看交接记录', action: 'handoffs' as const },
        ]
    wx.showActionSheet({
      itemList: actions.map(item => item.label),
      success: (result) => {
        const action = actions[result.tapIndex]?.action
        if (action === 'temporary') {
          this.openAddTemporaryStudent(operationId)
        }
        else if (action === 'notify') {
          this.openBatchNotify(operationId)
        }
        else if (action === 'handover') {
          void this.openHandover(operationId)
        }
        else if (action === 'handoffs') {
          void this.openViewHandoffs(operationId)
        }
      },
    })
  },
  handleStudentMoreActions(event: WechatMiniprogram.TouchEvent) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    const studentId = Number(event.currentTarget.dataset.studentId)
    const operation = this.data.operations.find(item => item.id === operationId)
    const student = operation?.students.find(item => item.student_id === studentId)
    if (!operation || !student || operation.status !== 'started' || this.data.loading || typeof wx === 'undefined') {
      return
    }
    const actions: Array<{ label: string, status?: Exclude<PickupMemberStatus, 'planned'>, photo?: boolean }> = student.status === 'planned'
      ? [
          { label: '自行到班', status: 'self_arrived' },
          { label: '家长接走', status: 'parent_picked_up' },
          { label: '请假', status: 'leave' },
          { label: '未找到', status: 'absent' },
          { label: '异常', status: 'abnormal' },
        ]
      : student.status === 'picked_up'
        ? [
            { label: '补传照片', photo: true },
            { label: '未到班', status: 'not_arrived' },
            { label: '异常', status: 'abnormal' },
          ]
        : [
            { label: '中途离班', status: 'midway_left' },
            { label: '异常', status: 'abnormal' },
          ]
    wx.showActionSheet({
      itemList: actions.map(item => item.label),
      success: (result) => {
        const action = actions[result.tapIndex]
        if (!action) {
          return
        }
        if (action.photo) {
          this.chooseStudentPhoto(operationId, studentId)
          return
        }
        if (action.status) {
          this.handleStudentStatus(operationId, studentId, action.status)
        }
      },
    })
  },
  handleMark(event: WechatMiniprogram.TouchEvent) {
    const operationId = Number(event.currentTarget.dataset.operationId)
    const studentId = Number(event.currentTarget.dataset.studentId)
    const status = event.currentTarget.dataset.status as Exclude<PickupMemberStatus, 'planned'>
    this.handleStudentStatus(operationId, studentId, status)
  },
  handleStudentStatus(operationId: number, studentId: number, status: Exclude<PickupMemberStatus, 'planned'>) {
    if (!operationId || !studentId || this.data.loading) {
      return
    }
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
    this.openAddTemporaryStudent(Number(event.currentTarget.dataset.operationId))
  },
  openAddTemporaryStudent(operationId: number) {
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
