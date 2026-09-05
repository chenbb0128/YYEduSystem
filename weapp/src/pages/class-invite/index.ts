import { getClassInviteQRCode } from '@/services/class-invites'
import { showFeedback } from '@/utils/feedback'

function decodeQuery(value: string | undefined) {
  if (!value) {
    return ''
  }
  try {
    return decodeURIComponent(value)
  }
  catch {
    return value
  }
}

function writeFile(filePath: string, data: ArrayBuffer) {
  if (typeof wx === 'undefined') {
    return Promise.reject(new Error('当前环境不支持保存二维码'))
  }
  return new Promise<void>((resolve, reject) => {
    wx.getFileSystemManager().writeFile({
      filePath,
      data,
      success: () => resolve(),
      fail: error => reject(new Error(error.errMsg || '二维码文件保存失败')),
    })
  })
}

Page({
  data: {
    schoolClassID: 0,
    classLabel: '当前班级',
    loading: false,
    saving: false,
    imagePath: '',
    errorMessage: '',
  },
  onLoad(options: Record<string, string | undefined> = {}) {
    const schoolClassID = Number(options.schoolClassId || 0)
    this.setData({
      schoolClassID: Number.isFinite(schoolClassID) ? schoolClassID : 0,
      classLabel: decodeQuery(options.classLabel) || '当前班级',
    })
    void this.loadQRCode()
  },
  async loadQRCode() {
    if (!this.data.schoolClassID) {
      this.setData({ errorMessage: '没有识别到要邀请的班级，请返回后重新选择。' })
      return
    }
    this.setData({ loading: true, errorMessage: '' })
    try {
      const image = await getClassInviteQRCode(this.data.schoolClassID)
      if (typeof wx === 'undefined') {
        throw new Error('当前环境不支持二维码预览')
      }
      const imagePath = `${wx.env.USER_DATA_PATH}/class-invite-${this.data.schoolClassID}.png`
      await writeFile(imagePath, image)
      this.setData({ imagePath })
    }
    catch (error) {
      this.setData({ errorMessage: error instanceof Error ? error.message : '二维码生成失败，请稍后重试' })
    }
    finally {
      this.setData({ loading: false })
    }
  },
  handlePreview() {
    if (typeof wx === 'undefined' || !this.data.imagePath) {
      return
    }
    wx.previewImage({ current: this.data.imagePath, urls: [this.data.imagePath] })
  },
  handleSave() {
    if (typeof wx === 'undefined' || !this.data.imagePath || this.data.saving) {
      return
    }
    this.setData({ saving: true })
    wx.saveImageToPhotosAlbum({
      filePath: this.data.imagePath,
      success: () => this.showToast('二维码已保存到手机相册'),
      fail: (error) => {
        if (/auth|permission|deny/i.test(error.errMsg || '')) {
          this.showToast('请允许保存图片到相册后再试')
          return
        }
        this.showToast(error.errMsg || '二维码保存失败')
      },
      complete: () => this.setData({ saving: false }),
    })
  },
  handleShareImage() {
    if (typeof wx === 'undefined' || !this.data.imagePath) {
      return
    }
    wx.showShareImageMenu({
      path: this.data.imagePath,
      needShowEntrance: false,
      success: () => this.showToast('已打开分享面板'),
      fail: error => this.showToast(error.errMsg || '分享二维码失败'),
    })
  },
  handleRetry() {
    void this.loadQRCode()
  },
  showToast(message: string) {
    showFeedback(this, message)
  },
})
