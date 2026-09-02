import { appEnv } from '@/config/env'
import { mediaURL } from '@/services/media'
import { request } from '@/services/request'
import { getAccessToken } from '@/utils/storage'

export type HomeworkStudentStatus = 'completed' | 'incomplete' | 'not_submitted' | 'pending'

export interface HomeworkTask {
  id: number
  homework_date: string
  school_id: number
  school_class_id: number
  subject: string
  content: string
  attachment_urls: string[]
  creator_name: string
  status: 'active' | 'cancelled'
  created_at: string
  updated_at: string
}

export interface HomeworkTaskStudent {
  id: number
  task_id: number
  student_id: number
  student_name: string
  status: HomeworkStudentStatus
  correction_note: string
  reviewed_at?: string
  created_at: string
  updated_at: string
}

interface ApiEnvelope<T> {
  code: number
  data: T
  message: string
}

interface PageResult<T> {
  items: T[]
  total: number
}

async function homeworkRequest<T>(url: string, method: 'GET' | 'POST' = 'GET', data?: unknown) {
  const response = await request<ApiEnvelope<T>>({ method, url, data })
  if (response.code !== 0) {
    throw new Error(response.message || '作业请求失败')
  }
  return response.data
}

export function getHomeworkTasks(date: string) {
  return homeworkRequest<PageResult<HomeworkTask>>(`/homework-tasks?date=${encodeURIComponent(date)}`)
}

export function createHomeworkTask(data: { homework_date: string, school_class_id: number, subject: string, content: string, attachment_urls?: string[], student_ids?: number[] }) {
  return homeworkRequest<HomeworkTask>('/homework-tasks', 'POST', data)
}

export function uploadHomeworkPhoto(filePath: string, formData: { task_id?: number } = {}) {
  return new Promise<string>((resolve, reject) => {
    if (typeof wx === 'undefined') {
      reject(new Error('当前环境不支持图片上传'))
      return
    }
    const uploadFields: Record<string, string> = {}
    for (const [key, value] of Object.entries(formData)) {
      if (typeof value === 'number' && value > 0) {
        uploadFields[key] = String(value)
      }
    }
    wx.uploadFile({
      filePath,
      name: 'file',
      formData: uploadFields,
      url: `${appEnv.apiBaseUrl}/uploads/homework`,
      header: getAccessToken() ? { Authorization: `Bearer ${getAccessToken()}` } : undefined,
      success: (result) => {
        try {
          const response = JSON.parse(result.data) as ApiEnvelope<{ url: string }>
          if (result.statusCode >= 200 && result.statusCode < 300 && response.code === 0 && response.data.url) {
            resolve(response.data.url)
            return
          }
          reject(new Error(response.message || '作业图片上传失败'))
        }
        catch (error) {
          reject(error)
        }
      },
      fail: reject,
    })
  })
}

export function getHomeworkTaskStudents(taskID: number) {
  return homeworkRequest<PageResult<HomeworkTaskStudent>>(`/homework-tasks/${taskID}/students`)
}

export function reviewHomeworkStudent(taskID: number, studentID: number, data: { status: Exclude<HomeworkStudentStatus, 'pending'>, correction_note?: string }) {
  return homeworkRequest<HomeworkTaskStudent>(`/homework-tasks/${taskID}/students/${studentID}/review`, 'POST', data)
}

export function homeworkStatusLabel(status: HomeworkStudentStatus) {
  return ({ completed: '已完成', incomplete: '需订正', not_submitted: '未提交', pending: '待批改' })[status]
}

export function homeworkPhotoURL(path?: string) {
  return mediaURL(path)
}
