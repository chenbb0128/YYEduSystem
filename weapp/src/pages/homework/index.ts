import type { HomeworkStudentStatus, HomeworkTask, HomeworkTaskStudent } from '@/services/homework'
import type { SchoolClassRecord, StudentRecord } from '@/services/master-data'
import { createHomeworkTask, getHomeworkTasks, getHomeworkTaskStudents, homeworkPhotoURL, homeworkStatusLabel, reviewHomeworkStudent, uploadHomeworkPhoto } from '@/services/homework'
import { getSchoolClasses, getStudents } from '@/services/master-data'
import { getToday } from '@/services/pickup'
import { getTeacherAssignments } from '@/services/teacher-assignments'
import { showFeedback } from '@/utils/feedback'

interface CandidateStudent extends StudentRecord {
  selected: boolean
}

interface HomeworkTaskView extends HomeworkTask {
  attachment_urls_signed: string[]
  class_label: string
}

interface HomeworkTaskStudentView extends HomeworkTaskStudent {
  status_class: string
  status_label: string
}

type HomeworkFormField = 'content' | 'subject'

Page({
  data: {
    date: getToday(),
    loading: false,
    submitting: false,
    showCreate: false,
    tasks: [] as HomeworkTaskView[],
    selectedTaskID: 0,
    selectedTask: null as HomeworkTaskView | null,
    taskStudents: [] as HomeworkTaskStudentView[],
    classes: [] as SchoolClassRecord[],
    classOptions: [] as string[],
    classIndex: 0,
    focusedField: '' as HomeworkFormField | '',
    allStudents: [] as StudentRecord[],
    candidateStudents: [] as CandidateStudent[],
    subject: '综合作业',
    content: '',
    attachmentURLs: [] as string[],
    attachmentNames: [] as string[],
    selectedStudentIDs: [] as number[],
  },
  onLoad() {
    void this.loadReferencesAndTasks()
  },
  onShow() {
    if (this.data.tasks.length) {
      void this.loadTasks()
    }
  },
  async loadReferencesAndTasks() {
    this.setData({ loading: true })
    try {
      const [classResult, studentResult, assignmentResult] = await Promise.all([getSchoolClasses(), getStudents(), getTeacherAssignments()])
      const assignedClassIDs = new Set(assignmentResult.items.filter(item => item.status === 'active').map(item => item.school_class_id))
      const classes = classResult.items.filter(item => item.status === 'active' && assignedClassIDs.has(item.id))
      const students = studentResult.items.filter(item => item.status === 'active' && assignedClassIDs.has(item.school_class_id))
      const classOptions = classes.map(item => `${item.grade}${item.name}`)
      this.setData({ classes, classOptions, allStudents: students })
      if (classes.length) {
        this.setData({ classIndex: 0 })
        this.updateCandidateStudents(classes[0].id, students)
      }
      await this.loadTasks()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '作业页面加载失败')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  async loadTasks() {
    try {
      const result = await getHomeworkTasks(this.data.date)
      const tasks = result.items.map(task => this.toTaskView(task))
      this.setData({ tasks })
      if (this.data.selectedTaskID) {
        const selectedTask = tasks.find(item => item.id === this.data.selectedTaskID)
        if (selectedTask) {
          await this.openTask(selectedTask)
        }
        else {
          this.setData({ selectedTaskID: 0, selectedTask: null, taskStudents: [] })
        }
      }
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '作业列表加载失败')
    }
  },
  updateCandidateStudents(classID: number, allStudents?: StudentRecord[]) {
    const students = allStudents || this.data.allStudents
    const selected = new Set(this.data.selectedStudentIDs)
    const candidateStudents = students.filter(item => item.school_class_id === classID).map(item => ({ ...item, selected: selected.has(item.id) }))
    const selectedStudentIDs = candidateStudents.filter(item => item.selected).map(item => item.id)
    this.setData({ candidateStudents, selectedStudentIDs })
  },
  handleDateChange(event: WechatMiniprogram.PickerChange) {
    this.setData({ date: event.detail.value as string, selectedTaskID: 0, selectedTask: null, taskStudents: [] })
    void this.loadTasks()
  },
  handleClassChange(event: WechatMiniprogram.PickerChange) {
    const classIndex = Number(event.detail.value)
    const selectedClass = this.data.classes[classIndex]
    if (!selectedClass) {
      return
    }
    this.setData({ classIndex, selectedStudentIDs: [] })
    this.updateCandidateStudents(selectedClass.id)
  },
  handleToggleStudent(event: WechatMiniprogram.TouchEvent) {
    const studentID = Number(event.currentTarget.dataset.studentId)
    const selectedStudentIDs = this.data.selectedStudentIDs.includes(studentID)
      ? this.data.selectedStudentIDs.filter(id => id !== studentID)
      : [...this.data.selectedStudentIDs, studentID]
    const candidateStudents = this.data.candidateStudents.map(item => ({ ...item, selected: selectedStudentIDs.includes(item.id) }))
    this.setData({ selectedStudentIDs, candidateStudents })
  },
  handleSelectAllStudents() {
    const selectedStudentIDs = this.data.candidateStudents.map(item => item.id)
    const candidateStudents = this.data.candidateStudents.map(item => ({ ...item, selected: true }))
    this.setData({ selectedStudentIDs, candidateStudents })
  },
  handleClearStudents() {
    const candidateStudents = this.data.candidateStudents.map(item => ({ ...item, selected: false }))
    this.setData({ selectedStudentIDs: [], candidateStudents })
  },
  handleInput(event: WechatMiniprogram.Input) {
    const field = event.currentTarget.dataset.field as HomeworkFormField
    this.setData({ [field]: event.detail.value })
  },
  handleFocus(event: WechatMiniprogram.InputFocus) {
    const field = event.currentTarget.dataset.field as HomeworkFormField
    this.setData({ focusedField: field })
  },
  handleBlur(event: WechatMiniprogram.InputBlur) {
    const field = event.currentTarget.dataset.field as HomeworkFormField
    if (this.data.focusedField === field) {
      this.setData({ focusedField: '' })
    }
  },
  openCreate() {
    if (!this.data.classes.length) {
      this.showToast('当前账号还没有负责班级')
      return
    }
    const selectedClass = this.data.classes[this.data.classIndex]
    this.setData({ showCreate: true, focusedField: '', subject: '综合作业', content: '', attachmentURLs: [], attachmentNames: [], selectedStudentIDs: [] })
    this.updateCandidateStudents(selectedClass.id)
    this.handleSelectAllStudents()
  },
  closeCreate() {
    this.setData({ showCreate: false })
  },
  chooseAttachments() {
    if (typeof wx === 'undefined') {
      return
    }
    const remaining = 9 - this.data.attachmentURLs.length
    if (remaining <= 0) {
      this.showToast('最多上传 9 张作业图片')
      return
    }
    wx.chooseMedia({
      count: remaining,
      mediaType: ['image'],
      sourceType: ['camera', 'album'],
      success: async (result) => {
        const files = result.tempFiles.filter(item => !!item.tempFilePath)
        if (!files.length) {
          return
        }
        this.setData({ submitting: true })
        try {
          const urls: string[] = []
          const names: string[] = []
          for (const file of files) {
            urls.push(await uploadHomeworkPhoto(file.tempFilePath))
            names.push(file.tempFilePath.split('/').pop() || '作业图片')
          }
          this.setData({ attachmentURLs: [...this.data.attachmentURLs, ...urls], attachmentNames: [...this.data.attachmentNames, ...names] })
          this.showToast(`已上传 ${files.length} 张图片`)
        }
        catch (error) {
          this.showToast(error instanceof Error ? error.message : '作业图片上传失败')
        }
        finally {
          this.setData({ submitting: false })
        }
      },
      fail: error => this.showToast(error.errMsg || '未选择图片'),
    })
  },
  async submitCreate() {
    const selectedClass = this.data.classes[this.data.classIndex]
    if (!selectedClass) {
      this.showToast('请选择学校班级')
      return
    }
    if (!this.data.content.trim()) {
      this.showToast('请填写作业内容')
      return
    }
    if (!this.data.selectedStudentIDs.length) {
      this.showToast('至少选择一名学生')
      return
    }
    this.setData({ submitting: true })
    try {
      const task = await createHomeworkTask({ homework_date: this.data.date, school_class_id: selectedClass.id, subject: this.data.subject.trim() || '综合作业', content: this.data.content.trim(), attachment_urls: this.data.attachmentURLs, student_ids: this.data.selectedStudentIDs })
      this.setData({ showCreate: false })
      this.showToast('作业已布置')
      await this.loadTasks()
      await this.openTask(task)
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '布置作业失败')
    }
    finally {
      this.setData({ submitting: false })
    }
  },
  handleOpenTask(event: WechatMiniprogram.TouchEvent) {
    const taskID = Number(event.currentTarget.dataset.taskId)
    const task = this.data.tasks.find(item => item.id === taskID)
    if (task) {
      void this.openTask(task)
    }
  },
  async openTask(task: HomeworkTask) {
    this.setData({ selectedTaskID: task.id, selectedTask: this.toTaskView(task), taskStudents: [] })
    try {
      const result = await getHomeworkTaskStudents(task.id)
      this.setData({ taskStudents: result.items.map(item => this.toTaskStudentView(item)) })
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '学生作业加载失败')
    }
  },
  handleReviewNote(event: WechatMiniprogram.Input) {
    const studentID = Number(event.currentTarget.dataset.studentId)
    const taskStudents = this.data.taskStudents.map(item => item.student_id === studentID ? { ...item, correction_note: event.detail.value } : item)
    this.setData({ taskStudents })
  },
  async reviewStudent(event: WechatMiniprogram.TouchEvent) {
    const taskID = this.data.selectedTaskID
    const studentID = Number(event.currentTarget.dataset.studentId)
    const status = event.currentTarget.dataset.status as Exclude<HomeworkStudentStatus, 'pending'>
    const student = this.data.taskStudents.find(item => item.student_id === studentID)
    if (!taskID || !student) {
      return
    }
    this.setData({ submitting: true })
    try {
      const updated = await reviewHomeworkStudent(taskID, studentID, { status, correction_note: student.correction_note })
      const taskStudents = this.data.taskStudents.map(item => item.student_id === studentID ? this.toTaskStudentView(updated) : item)
      this.setData({ taskStudents })
      this.showToast(`${student.student_name}：${homeworkStatusLabel(status)}`)
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '批改失败')
    }
    finally {
      this.setData({ submitting: false })
    }
  },
  classLabel(classID: number) {
    const item = this.data.classes.find(schoolClass => schoolClass.id === classID)
    return item ? `${item.grade}${item.name}` : `班级 #${classID}`
  },
  toTaskView(task: HomeworkTask): HomeworkTaskView {
    return { ...task, attachment_urls_signed: task.attachment_urls.map(url => homeworkPhotoURL(url)), class_label: this.classLabel(task.school_class_id) }
  },
  toTaskStudentView(student: HomeworkTaskStudent): HomeworkTaskStudentView {
    return { ...student, status_class: this.taskStatusClass(student.status), status_label: homeworkStatusLabel(student.status) }
  },
  taskStatusLabel(status: HomeworkStudentStatus) {
    return homeworkStatusLabel(status)
  },
  taskStatusClass(status: HomeworkStudentStatus) {
    if (status === 'completed') {
      return 'status-badge-success'
    }
    if (status === 'incomplete' || status === 'not_submitted') {
      return 'status-badge-danger'
    }
    return 'status-badge-warning'
  },
  homeworkPhotoURL,
  showToast(message: string) {
    showFeedback(this, message)
  },
})
