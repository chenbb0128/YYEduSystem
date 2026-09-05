import type { StudentRecord } from '@/services/master-data'
import type { ExtractedWrongQuestion, WrongPaper, WrongQuestion, WrongQuestionStatus } from '@/services/wrongbook'
import { uploadHomeworkPhoto } from '@/services/homework'
import { getStudents } from '@/services/master-data'
import { createParentWrongPaper, createWrongPaper, createWrongQuestions, extractWrongQuestions, getParentWrongPaper, getParentWrongPapers, getParentWrongQuestions, getWrongPaper, getWrongPapers, getWrongQuestions, updateWrongQuestion, wrongQuestionPhotoURL, wrongQuestionStatusLabel } from '@/services/wrongbook'
import { showFeedback } from '@/utils/feedback'

type WrongbookMode = 'parent' | 'teacher'
type WrongbookField = 'extractionText' | 'keyword' | 'subject'
type CandidateField = 'answer_text' | 'explanation' | 'knowledge_point' | 'question_text'

interface CandidateQuestion extends ExtractedWrongQuestion {
  selected: boolean
}

interface WrongQuestionView extends WrongQuestion {
  created_label: string
  selected_for_paper: boolean
  source_image_signed: string
  status_label: string
}

interface WrongPaperView extends WrongPaper {
  created_label: string
  source_label: string
}

function shortDate(value?: string) {
  if (!value) {
    return ''
  }
  return value.slice(0, 10)
}

function paperSourceLabel(source: WrongPaper['source']) {
  return ({ parent: '家长生成', system: '系统生成', teacher: '老师生成' })[source] || source
}

Page({
  data: {
    mode: 'teacher' as WrongbookMode,
    loading: false,
    submitting: false,
    students: [] as StudentRecord[],
    studentOptions: [] as string[],
    studentIndex: 0,
    selectedStudentID: 0,
    selectedStudentName: '',
    subject: '',
    keyword: '',
    statusFilter: 'active' as WrongQuestionStatus | '',
    statusOptions: ['待复习', '已掌握', '已归档', '全部'],
    sourceImageURL: '',
    sourceImageSigned: '',
    extractionText: '',
    extractionMocked: false,
    candidates: [] as CandidateQuestion[],
    questions: [] as WrongQuestionView[],
    papers: [] as WrongPaperView[],
    currentPaper: null as WrongPaper | null,
    paperVisible: false,
    focusedField: '' as WrongbookField | '',
  },
  onLoad(options: Record<string, string | undefined> = {}) {
    const mode = options.mode === 'parent' ? 'parent' : 'teacher'
    const selectedStudentID = Number(options.studentId || 0)
    this.setData({ mode, selectedStudentID: Number.isFinite(selectedStudentID) ? selectedStudentID : 0 })
    void this.bootstrap()
  },
  onShow() {
    if (this.data.selectedStudentID) {
      void this.loadData()
    }
  },
  async bootstrap() {
    if (this.data.mode === 'teacher') {
      await this.loadStudents()
      return
    }
    if (!this.data.selectedStudentID) {
      this.showToast('请先在家长端选择孩子')
      return
    }
    await this.loadData()
  },
  async loadStudents() {
    this.setData({ loading: true })
    try {
      const result = await getStudents()
      const students = result.items.filter(item => item.status === 'active')
      const selectedStudentID = this.data.selectedStudentID || students[0]?.id || 0
      const studentIndex = Math.max(0, students.findIndex(item => item.id === selectedStudentID))
      const selected = students[studentIndex]
      this.setData({
        students,
        studentOptions: students.map(item => item.name),
        studentIndex,
        selectedStudentID: selected?.id || 0,
        selectedStudentName: selected?.name || '',
      })
      await this.loadData()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '学生列表加载失败')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  async loadData() {
    this.setData({ loading: true })
    try {
      const params = { keyword: this.data.keyword.trim(), status: this.data.statusFilter, student_id: this.data.selectedStudentID || undefined, subject: this.data.subject.trim() || undefined }
      const [questionsResult, papersResult] = this.data.mode === 'parent'
        ? await Promise.all([getParentWrongQuestions(this.data.selectedStudentID, params), getParentWrongPapers(this.data.selectedStudentID)])
        : await Promise.all([getWrongQuestions(params), getWrongPapers({ student_id: this.data.selectedStudentID || undefined })])
      this.setData({
        questions: questionsResult.items.map(item => this.toQuestionView(item)),
        papers: papersResult.items.map(item => this.toPaperView(item)),
      })
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '错题集加载失败')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  handleStudentChange(event: WechatMiniprogram.PickerChange) {
    const studentIndex = Number(event.detail.value)
    const student = this.data.students[studentIndex]
    if (!student) {
      return
    }
    this.setData({ studentIndex, selectedStudentID: student.id, selectedStudentName: student.name, candidates: [], sourceImageURL: '', sourceImageSigned: '' })
    void this.loadData()
  },
  handleStatusChange(event: WechatMiniprogram.PickerChange) {
    const values: Array<WrongQuestionStatus | ''> = ['active', 'mastered', 'archived', '']
    this.setData({ statusFilter: values[Number(event.detail.value)] || 'active' })
    void this.loadData()
  },
  handleInput(event: WechatMiniprogram.Input) {
    const field = event.currentTarget.dataset.field as WrongbookField
    this.setData({ [field]: event.detail.value })
  },
  handleFocus(event: WechatMiniprogram.InputFocus) {
    this.setData({ focusedField: event.currentTarget.dataset.field as WrongbookField })
  },
  handleBlur(event: WechatMiniprogram.InputBlur) {
    const field = event.currentTarget.dataset.field as WrongbookField
    if (this.data.focusedField === field) {
      this.setData({ focusedField: '' })
    }
  },
  choosePhotoAndExtract() {
    if (this.data.mode !== 'teacher') {
      return
    }
    if (!this.data.selectedStudentID) {
      this.showToast('请先选择学生')
      return
    }
    if (typeof wx === 'undefined') {
      return
    }
    wx.chooseMedia({
      count: 1,
      mediaType: ['image'],
      sourceType: ['camera', 'album'],
      success: async (result) => {
        const filePath = result.tempFiles[0]?.tempFilePath
        if (!filePath) {
          return
        }
        this.setData({ submitting: true })
        try {
          const imageURL = await uploadHomeworkPhoto(filePath)
          this.setData({ sourceImageURL: imageURL, sourceImageSigned: wrongQuestionPhotoURL(imageURL) })
          await this.runExtract()
        }
        catch (error) {
          this.showToast(error instanceof Error ? error.message : '图片上传或提题失败')
        }
        finally {
          this.setData({ submitting: false })
        }
      },
      fail: error => this.showToast(error.errMsg || '未选择图片'),
    })
  },
  async runExtractFromText() {
    if (!this.data.selectedStudentID) {
      this.showToast('请先选择学生')
      return
    }
    if (!this.data.sourceImageURL && !this.data.extractionText.trim()) {
      this.showToast('请先拍照上传，或粘贴题目文字')
      return
    }
    this.setData({ submitting: true })
    try {
      await this.runExtract()
    }
    finally {
      this.setData({ submitting: false })
    }
  },
  async runExtract() {
    const result = await extractWrongQuestions({ image_url: this.data.sourceImageURL, source_text: this.data.extractionText.trim(), subject: this.data.subject.trim() || '综合' })
    this.setData({ extractionMocked: result.mocked, candidates: result.items.map(item => ({ ...item, selected: true })) })
    this.showToast(result.mocked ? '已生成待校对题目卡片' : `已提取 ${result.total} 道候选题`)
  },
  handleCandidateToggle(event: WechatMiniprogram.TouchEvent) {
    const index = Number(event.currentTarget.dataset.index)
    const candidates = this.data.candidates.map((item, itemIndex) => itemIndex === index ? { ...item, selected: !item.selected } : item)
    this.setData({ candidates })
  },
  handleCandidateInput(event: WechatMiniprogram.Input) {
    const index = Number(event.currentTarget.dataset.index)
    const field = event.currentTarget.dataset.field as CandidateField
    const candidates = this.data.candidates.map((item, itemIndex) => itemIndex === index ? { ...item, [field]: event.detail.value } : item)
    this.setData({ candidates })
  },
  async saveSelectedCandidates() {
    const selected = this.data.candidates.filter(item => item.selected && item.question_text.trim())
    if (!this.data.selectedStudentID) {
      this.showToast('请先选择学生')
      return
    }
    if (!selected.length) {
      this.showToast('请至少选择一道错题')
      return
    }
    this.setData({ submitting: true })
    try {
      const result = await createWrongQuestions(selected.map(item => ({
        answer_text: item.answer_text.trim(),
        explanation: item.explanation.trim(),
        knowledge_point: item.knowledge_point.trim(),
        question_text: item.question_text.trim(),
        source_image_url: this.data.sourceImageURL,
        student_id: this.data.selectedStudentID,
        subject: item.subject || this.data.subject || '综合',
        teacher_note: this.data.extractionMocked ? 'OCR 未配置，已由老师校对保存。' : '',
      })))
      this.setData({ candidates: [], extractionText: '', extractionMocked: false })
      this.showToast(`已保存 ${result.total} 道错题`)
      await this.loadData()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '保存错题失败')
    }
    finally {
      this.setData({ submitting: false })
    }
  },
  handleQuestionSelect(event: WechatMiniprogram.TouchEvent) {
    const id = Number(event.currentTarget.dataset.id)
    const questions = this.data.questions.map(item => item.id === id ? { ...item, selected_for_paper: !item.selected_for_paper } : item)
    this.setData({ questions })
  },
  async handleQuestionStatus(event: WechatMiniprogram.TouchEvent) {
    const id = Number(event.currentTarget.dataset.id)
    const status = event.currentTarget.dataset.status as WrongQuestionStatus
    const item = this.data.questions.find(question => question.id === id)
    if (!item || this.data.mode !== 'teacher') {
      return
    }
    this.setData({ submitting: true })
    try {
      await updateWrongQuestion(id, {
        answer_text: item.answer_text,
        explanation: item.explanation,
        knowledge_point: item.knowledge_point,
        question_text: item.question_text,
        status,
        subject: item.subject,
        teacher_note: item.teacher_note,
      })
      this.showToast(status === 'mastered' ? '已标记掌握' : status === 'archived' ? '已归档' : '已恢复待复习')
      await this.loadData()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '状态更新失败')
    }
    finally {
      this.setData({ submitting: false })
    }
  },
  handleSearch() {
    void this.loadData()
  },
  handleClearSearch() {
    this.setData({ keyword: '', statusFilter: 'active' })
    void this.loadData()
  },
  async handleGeneratePaper() {
    const questionIDs = this.data.questions.filter(item => item.selected_for_paper).map(item => item.id)
    if (!this.data.selectedStudentID) {
      this.showToast('请先选择学生')
      return
    }
    if (!questionIDs.length) {
      this.showToast('请先勾选错题')
      return
    }
    const generate = async (title = '') => {
      this.setData({ submitting: true })
      try {
        const paper = this.data.mode === 'parent'
          ? await createParentWrongPaper(this.data.selectedStudentID, { question_ids: questionIDs, title })
          : await createWrongPaper({ question_ids: questionIDs, student_id: this.data.selectedStudentID, title })
        this.setData({ currentPaper: paper, paperVisible: true, questions: this.data.questions.map(item => ({ ...item, selected_for_paper: false })) })
        this.showToast('复习卷已生成')
        await this.loadData()
      }
      catch (error) {
        this.showToast(error instanceof Error ? error.message : '生成复习卷失败')
      }
      finally {
        this.setData({ submitting: false })
      }
    }
    if (typeof wx === 'undefined') {
      await generate()
      return
    }
    wx.showModal({
      title: '生成复习卷',
      editable: true,
      placeholderText: '可填写复习卷标题',
      content: `${this.data.selectedStudentName || '学生'}错题复习卷`,
      success: (result) => {
        if (result.confirm) {
          void generate((result.content || '').trim())
        }
      },
    })
  },
  async handleOpenPaper(event: WechatMiniprogram.TouchEvent) {
    const id = Number(event.currentTarget.dataset.id)
    if (!id) {
      return
    }
    this.setData({ submitting: true })
    try {
      const paper = this.data.mode === 'parent'
        ? await getParentWrongPaper(this.data.selectedStudentID, id)
        : await getWrongPaper(id)
      this.setData({ currentPaper: paper, paperVisible: true })
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '复习卷加载失败')
    }
    finally {
      this.setData({ submitting: false })
    }
  },
  closePaper() {
    this.setData({ paperVisible: false })
  },
  toQuestionView(item: WrongQuestion): WrongQuestionView {
    return { ...item, created_label: shortDate(item.created_at), selected_for_paper: false, source_image_signed: wrongQuestionPhotoURL(item.source_image_url), status_label: wrongQuestionStatusLabel(item.status) }
  },
  toPaperView(item: WrongPaper): WrongPaperView {
    return { ...item, created_label: shortDate(item.created_at), source_label: paperSourceLabel(item.source) }
  },
  showToast(message: string) {
    showFeedback(this, message)
  },
})
