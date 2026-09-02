import type { StudentRecord } from '@/services/master-data'
import type { DailySummary } from '@/services/summary'

import { getStudents } from '@/services/master-data'
import { getToday } from '@/services/pickup'
import { closeDailySummary, correctDailySummary, generateDailySummary, getDailySummaries, getDailySummaryVersions, publishDailySummary, updateDailySummary, withdrawDailySummary } from '@/services/summary'
import { showFeedback } from '@/utils/feedback'

Page({
  data: {
    loading: false,
    submitting: false,
    date: getToday(),
    summary: null as DailySummary | null,
    content: '',
    students: [] as StudentRecord[],
    childUpdates: {} as Record<string, string>,
    childUpdateItems: [] as Array<{ student_id: number, student_name: string, note: string }>,
    versions: [] as Array<{ id: number, version: number, action: string, reason?: string, created_by_name: string, created_at: string }>,
    correctionVisible: false,
    correctionContent: '',
    correctionReason: '',
    correctionChildUpdates: {} as Record<string, string>,
  },
  onLoad() {
    void this.load()
  },
  onShow() {
    void this.load()
  },
  async load() {
    this.setData({ loading: true })
    try {
      const [result, studentResult] = await Promise.all([getDailySummaries(this.data.date), getStudents()])
      const summary = result.items[0] || null
      const childUpdates = summary?.child_updates || {}
      let versions: typeof this.data.versions = []
      if (summary) {
        try {
          versions = (await getDailySummaryVersions(summary.id)).items
        }
        catch {}
      }
      this.setData({ summary, content: summary?.content || '', students: studentResult.items, childUpdates, childUpdateItems: this.toChildUpdateItems(childUpdates, studentResult.items), versions, correctionVisible: false, correctionChildUpdates: childUpdates })
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '每日总结加载失败')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  handleDateChange(event: WechatMiniprogram.PickerChange) {
    this.setData({ date: event.detail.value as string })
    void this.load()
  },
  handleInput(event: WechatMiniprogram.Input) {
    this.setData({ content: event.detail.value })
  },
  handleChildUpdate(event: WechatMiniprogram.Input) {
    const studentID = String(event.currentTarget.dataset.studentId)
    const childUpdates = { ...this.data.childUpdates, [studentID]: event.detail.value }
    this.setData({ childUpdates, childUpdateItems: this.toChildUpdateItems(childUpdates, this.data.students) })
  },
  async generate() {
    if (this.data.submitting || (this.data.summary && this.data.summary.status !== 'draft')) {
      return
    }
    this.setData({ submitting: true })
    try {
      const summary = await generateDailySummary(this.data.date)
      const childUpdates = summary.child_updates || {}
      this.setData({ summary, content: summary.content, childUpdates, childUpdateItems: this.toChildUpdateItems(childUpdates, this.data.students) })
      this.showToast('已生成总结草稿')
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '生成总结失败')
    }
    finally {
      this.setData({ submitting: false })
    }
  },
  async save(): Promise<boolean> {
    if (!this.data.summary || this.data.summary.status !== 'draft' || !this.data.content.trim()) {
      this.showToast('请先生成并填写总结内容')
      return false
    }
    if (this.data.submitting) {
      return false
    }
    this.setData({ submitting: true })
    try {
      const summary = await updateDailySummary(this.data.summary.id, {
        content: this.data.content.trim(),
        child_updates: this.data.childUpdates,
      })
      const childUpdates = summary.child_updates || {}
      this.setData({ summary, content: summary.content, childUpdates, childUpdateItems: this.toChildUpdateItems(childUpdates, this.data.students) })
      this.showToast('总结已保存')
      return true
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '保存总结失败')
      return false
    }
    finally {
      this.setData({ submitting: false })
    }
  },
  async publish() {
    if (!this.data.summary || this.data.summary.status !== 'draft' || this.data.submitting) {
      this.showToast('当前总结不能发布')
      return
    }
    const saved = await this.save()
    if (!saved || !this.data.summary) {
      return
    }
    this.setData({ submitting: true })
    try {
      const summary = await publishDailySummary(this.data.summary.id)
      this.setData({ summary })
      this.showToast('总结已发布，家长可查看')
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '发布总结失败')
    }
    finally {
      this.setData({ submitting: false })
    }
  },
  openCorrection() {
    if (!this.data.summary || !['published', 'closed', 'withdrawn'].includes(this.data.summary.status)) {
      return
    }
    this.setData({ correctionVisible: true, correctionContent: this.data.content, correctionReason: '', correctionChildUpdates: { ...this.data.childUpdates } })
  },
  handleCorrectionInput(event: WechatMiniprogram.Input) {
    const field = event.currentTarget.dataset.field as 'correctionContent' | 'correctionReason'
    this.setData({ [field]: event.detail.value })
  },
  handleCorrectionChildUpdate(event: WechatMiniprogram.Input) {
    const studentID = String(event.currentTarget.dataset.studentId)
    this.setData({ correctionChildUpdates: { ...this.data.correctionChildUpdates, [studentID]: event.detail.value } })
  },
  async correct() {
    if (!this.data.summary || !this.data.correctionContent.trim() || !this.data.correctionReason.trim() || this.data.submitting) {
      this.showToast('请填写更正内容和原因')
      return
    }
    this.setData({ submitting: true })
    try {
      const summary = await correctDailySummary(this.data.summary.id, { content: this.data.correctionContent.trim(), child_updates: this.data.correctionChildUpdates, reason: this.data.correctionReason.trim() })
      const childUpdates = summary.child_updates || {}
      this.setData({ summary, content: summary.content, childUpdates, childUpdateItems: this.toChildUpdateItems(childUpdates, this.data.students), correctionVisible: false })
      this.showToast('更正已发布')
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '更正发布失败')
    }
    finally {
      this.setData({ submitting: false })
    }
  },
  async withdraw() {
    if (!this.data.summary || this.data.summary.status !== 'published' || this.data.submitting) {
      return
    }
    this.showToast('请在下方填写撤回原因后操作')
    this.setData({ correctionVisible: true, correctionContent: this.data.content, correctionReason: '', correctionChildUpdates: { ...this.data.childUpdates } })
  },
  async submitWithdraw() {
    if (!this.data.summary || this.data.summary.status !== 'published' || !this.data.correctionReason.trim() || this.data.submitting) {
      this.showToast('请填写撤回原因')
      return
    }
    this.setData({ submitting: true })
    try {
      const summary = await withdrawDailySummary(this.data.summary.id, this.data.correctionReason.trim())
      this.setData({ summary, correctionVisible: false })
      this.showToast('总结已撤回')
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '撤回失败')
    }
    finally {
      this.setData({ submitting: false })
    }
  },
  async close() {
    if (!this.data.summary || this.data.summary.status !== 'published' || this.data.submitting) {
      return
    }
    this.setData({ submitting: true })
    try {
      const summary = await closeDailySummary(this.data.summary.id)
      this.setData({ summary })
      this.showToast('当天托管已结束')
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '结束当天工作失败')
    }
    finally {
      this.setData({ submitting: false })
    }
  },
  statusLabel(status?: string) {
    return ({ draft: '草稿', published: '已发布', closed: '已结束', withdrawn: '已撤回' } as Record<string, string>)[status || ''] || '未生成'
  },
  versionActionLabel(action: string) {
    return ({ corrected: '更正发布', generated: '生成草稿', updated: '保存修改', published: '发布', closed: '结束当天', withdrawn: '撤回' } as Record<string, string>)[action] || action
  },
  toChildUpdateItems(updates: Record<string, string>, students: StudentRecord[]) {
    return Object.entries(updates).map(([studentID, note]) => ({
      student_id: Number(studentID),
      student_name: students.find(item => item.id === Number(studentID))?.name || `学生 #${studentID}`,
      note,
    }))
  },
  showToast(message: string) {
    showFeedback(this, message)
  },
})
