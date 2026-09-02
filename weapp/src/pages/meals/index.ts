import type { StudentRecord } from '@/services/master-data'

import type { DietNoteChangeRequest, MealPlan } from '@/services/meal'
import { getStudents } from '@/services/master-data'
import { copyMeal, getDietNoteChangeRequests, getDietNotes, getMeals, mealPhotoURL, reviewDietNoteChangeRequest, uploadMealPhoto, upsertMeal } from '@/services/meal'
import { getToday } from '@/services/pickup'
import { showFeedback } from '@/utils/feedback'

type MealView = MealPlan & { photo_url_signed: string }
interface DietNoteView { student_id: number, student_name: string, note: string, updated_at: string }
interface DietNoteChangeRequestView extends DietNoteChangeRequest { student_name: string }
interface WeekMenuView { meal_date: string, weekday_label: string, menu_text: string, adjustment_note: string, photo_url_signed: string, recorded: boolean }

function offsetDate(date: string, offset: number) {
  const value = new Date(`${date}T00:00:00`)
  value.setDate(value.getDate() + offset)
  const month = `${value.getMonth() + 1}`.padStart(2, '0')
  const day = `${value.getDate()}`.padStart(2, '0')
  return `${value.getFullYear()}-${month}-${day}`
}

function weekStart(date: string) {
  const value = new Date(`${date}T00:00:00`)
  const mondayOffset = (value.getDay() + 6) % 7
  value.setDate(value.getDate() - mondayOffset)
  return value
}

function formatDate(value: Date) {
  const month = `${value.getMonth() + 1}`.padStart(2, '0')
  const day = `${value.getDate()}`.padStart(2, '0')
  return `${value.getFullYear()}-${month}-${day}`
}

function buildWeekMenu(date: string, plans: MealPlan[]): WeekMenuView[] {
  const start = weekStart(date)
  const weekdays = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
  return Array.from({ length: 7 }, (_, index) => {
    const current = new Date(start)
    current.setDate(current.getDate() + index)
    const mealDate = formatDate(current)
    const plan = plans.find(item => item.meal_date === mealDate)
    return {
      meal_date: mealDate,
      weekday_label: weekdays[current.getDay()],
      menu_text: plan?.menu_text || '',
      adjustment_note: plan?.adjustment_note || '',
      photo_url_signed: mealPhotoURL(plan?.photo_url),
      recorded: Boolean(plan),
    }
  })
}

Page({
  data: {
    loading: false,
    submitting: false,
    date: getToday(),
    plans: [] as MealView[],
    weekMenu: [] as WeekMenuView[],
    historyPlans: [] as MealView[],
    dietNotes: [] as DietNoteView[],
    dietNoteRequests: [] as DietNoteChangeRequestView[],
    menuText: '',
    adjustmentNote: '',
    photoURL: '',
    photoPreviewURL: '',
    focusedField: '' as 'menuText' | 'adjustmentNote' | '',
    showEditor: false,
  },
  onLoad() {
    void this.loadPlans()
  },
  onShow() {
    void this.loadPlans()
  },
  async loadPlans() {
    this.setData({ loading: true })
    try {
      const start = weekStart(this.data.date)
      const weekFrom = formatDate(start)
      const weekEnd = new Date(start)
      weekEnd.setDate(weekEnd.getDate() + 6)
      const [result, weekResult, notes, students, dietNoteRequests] = await Promise.all([getMeals({ from: offsetDate(this.data.date, -6), to: this.data.date }), getMeals({ from: weekFrom, to: formatDate(weekEnd) }), getDietNotes(), getStudents(), getDietNoteChangeRequests({ status: 'pending' })])
      const historyPlans = result.items.map(item => ({
        ...item,
        photo_url_signed: mealPhotoURL(item.photo_url),
      }))
      this.setData({
        plans: historyPlans.filter(item => item.meal_date === this.data.date),
        weekMenu: buildWeekMenu(this.data.date, weekResult.items),
        historyPlans,
        dietNotes: notes.items.filter(item => item.note.trim()).map(item => ({ ...item, student_name: students.items.find((student: StudentRecord) => student.id === item.student_id)?.name || `学生 #${item.student_id}` })),
        dietNoteRequests: dietNoteRequests.items.map(item => ({ ...item, student_name: students.items.find((student: StudentRecord) => student.id === item.student_id)?.name || `学生 #${item.student_id}` })),
      })
    }
    catch (error) {
      this.setData({ plans: [], weekMenu: [], historyPlans: [], dietNotes: [], dietNoteRequests: [] })
      this.showToast(error instanceof Error ? error.message : '餐食加载失败')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  handleDateChange(event: WechatMiniprogram.PickerChange) {
    this.setData({ date: event.detail.value as string })
    void this.loadPlans()
  },
  handleWeekMenuSelect(event: WechatMiniprogram.TouchEvent) {
    const date = String(event.currentTarget.dataset.date || '')
    if (!date || date === this.data.date) {
      return
    }
    this.setData({ date })
    void this.loadPlans()
  },
  openEditor() {
    const current = this.data.plans[0]
    this.setData({
      showEditor: true,
      menuText: current?.menu_text || '',
      adjustmentNote: current?.adjustment_note || '',
      photoURL: current?.photo_url || '',
      photoPreviewURL: mealPhotoURL(current?.photo_url),
      focusedField: '',
    })
  },
  closeEditor() {
    this.setData({ showEditor: false })
  },
  handleInput(event: WechatMiniprogram.Input) {
    this.setData({ [event.currentTarget.dataset.field]: event.detail.value })
  },
  handleFocus(event: WechatMiniprogram.InputFocus) {
    this.setData({ focusedField: event.currentTarget.dataset.field })
  },
  handleBlur() {
    this.setData({ focusedField: '' })
  },
  choosePhoto() {
    if (typeof wx === 'undefined') {
      return
    }
    wx.chooseMedia({
      count: 1,
      mediaType: ['image'],
      sourceType: ['camera', 'album'],
      success: async (result) => {
        const path = result.tempFiles[0]?.tempFilePath
        if (!path) {
          return
        }
        this.setData({ submitting: true })
        try {
          const url = await uploadMealPhoto(path, { meal_plan_id: this.data.plans[0]?.id })
          this.setData({ photoURL: url, photoPreviewURL: mealPhotoURL(url) })
          this.showToast('餐食照片已上传')
        }
        catch (error) {
          this.showToast(error instanceof Error ? error.message : '餐食照片上传失败')
        }
        finally {
          this.setData({ submitting: false })
        }
      },
      fail: error => this.showToast(error.errMsg || '未选择照片'),
    })
  },
  async submit() {
    if (!this.data.menuText.trim()) {
      this.showToast('请填写菜单内容')
      return
    }
    this.setData({ submitting: true })
    try {
      await upsertMeal({
        meal_date: this.data.date,
        menu_text: this.data.menuText.trim(),
        adjustment_note: this.data.adjustmentNote.trim(),
        photo_url: this.data.photoURL,
      })
      this.showToast('餐食已保存')
      this.closeEditor()
      await this.loadPlans()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '餐食保存失败')
    }
    finally {
      this.setData({ submitting: false })
    }
  },
  async copyHistory() {
    const source = await this.pickDate('选择要复制的历史日期')
    if (!source || source === this.data.date) {
      return
    }
    this.setData({ submitting: true })
    try {
      await copyMeal({ source_date: source, target_date: this.data.date })
      this.showToast('历史菜单已复制')
      await this.loadPlans()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '复制菜单失败')
    }
    finally {
      this.setData({ submitting: false })
    }
  },
  reviewDietNoteRequest(event: WechatMiniprogram.TouchEvent) {
    const requestID = Number(event.currentTarget.dataset.requestId)
    const status = String(event.currentTarget.dataset.status) as 'approved' | 'rejected'
    if (!requestID || (status !== 'approved' && status !== 'rejected') || typeof wx === 'undefined') {
      return
    }
    const title = status === 'approved' ? '确认通过饮食备注' : '驳回饮食备注申请'
    wx.showModal({
      title,
      editable: true,
      placeholderText: status === 'approved' ? '可填写照护提醒（可选）' : '请填写未通过原因（可选）',
      success: (result) => {
        if (!result.confirm) {
          return
        }
        void this.submitDietNoteReview(requestID, status, (result.content || '').trim())
      },
    })
  },
  async submitDietNoteReview(requestID: number, status: 'approved' | 'rejected', reviewNote: string) {
    this.setData({ submitting: true })
    try {
      await reviewDietNoteChangeRequest(requestID, { status, review_note: reviewNote })
      this.showToast(status === 'approved' ? '饮食备注已确认生效' : '申请已驳回')
      await this.loadPlans()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '审核失败，请刷新后重试')
    }
    finally {
      this.setData({ submitting: false })
    }
  },
  pickDate(title: string): Promise<string | null> {
    return new Promise((resolve) => {
      if (typeof wx === 'undefined') {
        resolve(null)
        return
      }
      wx.showModal({
        title,
        editable: true,
        placeholderText: '请输入日期，例如 2026-09-01',
        success: result => resolve(result.confirm ? (result.content || '').trim() : null),
        fail: () => resolve(null),
      })
    })
  },
  showToast(message: string) {
    showFeedback(this, message)
  },
})
