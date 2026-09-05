import { request } from '@/services/request'

export interface ClassInviteView {
  token: string
  school_class_id: number
  school_name: string
  grade: string
  class_name: string
  label: string
}

interface ApiEnvelope<T> {
  code: number
  data: T
  message: string
}

export function getClassInvite(token: string) {
  return request<ApiEnvelope<ClassInviteView>>({ method: 'GET', url: `/parent/class-invites/${encodeURIComponent(token)}` }).then((response) => {
    if (response.code !== 0) {
      throw new Error(response.message || '班级邀请加载失败')
    }
    return response.data
  })
}

export function getClassInviteQRCode(schoolClassID: number) {
  return request<ArrayBuffer>({
    method: 'GET',
    url: '/class-invites/qrcode',
    params: { school_class_id: schoolClassID },
    responseType: 'arraybuffer',
  })
}

export function savePendingClassInviteToken(token: string) {
  if (typeof wx !== 'undefined' && token) {
    wx.setStorageSync('parent.pending-class-invite-token', token)
  }
}

export function getPendingClassInviteToken() {
  if (typeof wx === 'undefined') {
    return ''
  }
  return wx.getStorageSync('parent.pending-class-invite-token') || ''
}

export function clearPendingClassInviteToken() {
  if (typeof wx !== 'undefined') {
    wx.removeStorageSync('parent.pending-class-invite-token')
  }
}
