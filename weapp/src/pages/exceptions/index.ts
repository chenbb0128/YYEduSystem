import type { DailyException, DailyExceptionCategory } from '@/services/report'
import { getToday } from '@/services/pickup'
import { acknowledgeDailyException, dailyExceptionCategoryLabel, getDailyExceptions } from '@/services/report'
import { showFeedback } from '@/utils/feedback'

interface ExceptionView extends DailyException {
  category_label: string
}

interface CategoryView {
  key: 'all' | DailyExceptionCategory
  label: string
  count: number
  active: boolean
}

const categories: Array<{ key: DailyExceptionCategory, label: string }> = [
  { key: 'pickup', label: '接送' },
  { key: 'homework', label: '作业' },
  { key: 'meal', label: '餐食' },
  { key: 'leave', label: '请假' },
  { key: 'application', label: '入班申请' },
  { key: 'summary', label: '每日总结' },
  { key: 'student', label: '学生档案' },
]

let loadPromise: Promise<void> | undefined

Page({
  data: {
    date: getToday(),
    loading: false,
    acknowledging: false,
    loadError: '',
    acknowledgeVisible: false,
    acknowledgeException: null as ExceptionView | null,
    acknowledgeNote: '',
    showAcknowledged: false,
    selectedCategory: 'all' as 'all' | DailyExceptionCategory,
    total: 0,
    dangerCount: 0,
    items: [] as ExceptionView[],
    visibleItems: [] as ExceptionView[],
    categoryItems: [] as CategoryView[],
  },
  onLoad() {
    void this.load()
  },
  onShow() {
    void this.load()
  },
  async onPullDownRefresh() {
    try {
      await this.load()
    }
    finally {
      if (typeof wx !== 'undefined') {
        wx.stopPullDownRefresh()
      }
    }
  },
  async load() {
    if (loadPromise) {
      return loadPromise
    }
    loadPromise = this.loadInternal()
    try {
      await loadPromise
    }
    finally {
      loadPromise = undefined
    }
  },
  async loadInternal() {
    this.setData({ loading: true })
    try {
      const result = await getDailyExceptions(this.data.date, this.data.showAcknowledged)
      const items = result.items.map(item => ({ ...item, category_label: dailyExceptionCategoryLabel(item.category) }))
      const dangerCount = items.filter(item => item.severity === 'danger').length
      const categoryItems: CategoryView[] = [
        { key: 'all', label: '全部', count: items.length, active: this.data.selectedCategory === 'all' },
        ...categories.map(item => ({ key: item.key, label: item.label, count: result.counts[item.key] || 0, active: this.data.selectedCategory === item.key })),
      ]
      this.setData({ total: items.length, dangerCount, items, categoryItems, loadError: '' })
      this.applyFilter(this.data.selectedCategory, items, categoryItems)
    }
    catch (error) {
      this.setData({ loadError: error instanceof Error ? error.message : '异常数据加载失败' })
      this.showToast(error instanceof Error ? error.message : '异常数据加载失败')
    }
    finally {
      this.setData({ loading: false })
    }
  },
  handleCategoryChange(event: WechatMiniprogram.TouchEvent) {
    const category = event.currentTarget.dataset.category as 'all' | DailyExceptionCategory
    if (!category) {
      return
    }
    this.applyFilter(category)
  },
  applyFilter(category: 'all' | DailyExceptionCategory, source?: ExceptionView[], sourceCategories?: CategoryView[]) {
    const sourceItems = source || this.data.items
    const currentCategories = sourceCategories || this.data.categoryItems
    const visibleItems = category === 'all' ? sourceItems : sourceItems.filter(item => item.category === category)
    const categoryItems = currentCategories.map(item => ({ ...item, active: item.key === category }))
    this.setData({ selectedCategory: category, visibleItems, categoryItems })
  },
  handleOpenException(event: WechatMiniprogram.TouchEvent) {
    const action = String(event.currentTarget.dataset.action || '')
    if (!action || typeof wx === 'undefined') {
      return
    }
    wx.navigateTo({ url: action })
  },
  handleToggleAcknowledged() {
    if (this.data.loading || this.data.acknowledging) {
      return
    }
    const showAcknowledged = !this.data.showAcknowledged
    this.setData({ showAcknowledged })
    void this.load()
  },
  handleOpenAcknowledge(event: WechatMiniprogram.TouchEvent) {
    const exceptionID = String(event.currentTarget.dataset.exceptionId || '')
    const exception = this.data.items.find(item => item.id === exceptionID)
    if (!exception || exception.acknowledged || this.data.acknowledging) {
      return
    }
    this.setData({ acknowledgeVisible: true, acknowledgeException: exception, acknowledgeNote: '' })
  },
  handleAcknowledgeInput(event: WechatMiniprogram.Input) {
    this.setData({ acknowledgeNote: event.detail.value })
  },
  closeAcknowledge() {
    if (this.data.acknowledging) {
      return
    }
    this.setData({ acknowledgeVisible: false, acknowledgeException: null, acknowledgeNote: '' })
  },
  async submitAcknowledge() {
    const exception = this.data.acknowledgeException
    if (!exception || this.data.acknowledging) {
      return
    }
    this.setData({ acknowledging: true })
    try {
      await acknowledgeDailyException(exception.id, this.data.date, this.data.acknowledgeNote)
      this.setData({ acknowledgeVisible: false, acknowledgeException: null, acknowledgeNote: '' })
      this.showToast('已标记为已知并记录处理说明')
      await this.load()
    }
    catch (error) {
      this.showToast(error instanceof Error ? error.message : '异常处理失败，请重试')
    }
    finally {
      this.setData({ acknowledging: false })
    }
  },
  showToast(message: string) {
    showFeedback(this, message)
  },
})
