import Toast from 'tdesign-miniprogram/toast/index'

type PageContext = WechatMiniprogram.Page.TrivialInstance | WechatMiniprogram.Component.TrivialInstance

export function showFeedback(context: PageContext, message: string) {
  if (typeof wx !== 'undefined' && typeof wx.showToast === 'function') {
    wx.showToast({ title: message, icon: 'none' })
    return
  }

  try {
    Toast({ context, selector: '#t-toast', message })
  }
  catch {
    // The web preview does not expose the native component instance.
    // eslint-disable-next-line no-console
    console.info(`[feedback] ${message}`)
  }
}
