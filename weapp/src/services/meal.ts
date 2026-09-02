import { appEnv } from '@/config/env'
import { mediaURL } from '@/services/media'
import { request } from '@/services/request'
import { getAccessToken } from '@/utils/storage'

export interface MealPlan {
  id: number
  meal_date: string
  menu_text: string
  photo_url?: string
  adjustment_note: string
  created_by_name: string
  status: 'active' | 'closed'
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

async function mealRequest<T>(
  url: string,
  method: 'GET' | 'POST' | 'PUT' = 'GET',
  data?: unknown,
  params?: Record<string, string>,
) {
  const response = await request<ApiEnvelope<T>>({ method, url, data, params })
  if (response.code !== 0) {
    throw new Error(response.message || '餐食请求失败')
  }
  return response.data
}

export function getMeals(params?: { from?: string, to?: string }) {
  return mealRequest<PageResult<MealPlan>>('/meals', 'GET', undefined, params)
}

export function getParentMeals(date: string) {
  return mealRequest<PageResult<MealPlan>>('/parent/meals', 'GET', undefined, { date })
}

export function getParentMealHistory(from: string, to: string) {
  return mealRequest<PageResult<MealPlan>>('/parent/meals', 'GET', undefined, { from, to })
}

export interface ParentDietNote {
  id: number
  student_id: number
  note: string
  updated_by_name: string
  updated_at: string
}

export interface DietNoteChangeRequest {
  id: number
  student_id: number
  parent_account_id?: number
  current_note: string
  requested_note: string
  status: 'pending' | 'approved' | 'rejected'
  review_note: string
  reviewed_at?: string
  created_at: string
  updated_at: string
}

export type DietNote = ParentDietNote

export function getDietNotes(studentID?: number) {
  return mealRequest<PageResult<DietNote>>('/meal-diet-notes', 'GET', undefined, studentID ? { student_id: String(studentID) } : undefined)
}

export function getParentDietNote(studentID: number) {
  return mealRequest<ParentDietNote | null>(`/parent/students/${studentID}/diet-note`)
}

export function updateParentDietNote(studentID: number, note: string) {
  return mealRequest<DietNoteChangeRequest>(`/parent/students/${studentID}/diet-note`, 'PUT', { note })
}

export function getParentDietNoteChangeRequests(studentID: number) {
  return mealRequest<{ items: DietNoteChangeRequest[], total: number }>(`/parent/students/${studentID}/diet-note-requests`)
}

export function createParentDietNoteChangeRequest(studentID: number, note: string) {
  return mealRequest<DietNoteChangeRequest>(`/parent/students/${studentID}/diet-note-requests`, 'POST', { note })
}

export function getDietNoteChangeRequests(params?: { student_id?: number, status?: DietNoteChangeRequest['status'] }) {
  const query: Record<string, string> = {}
  if (params?.student_id) {
    query.student_id = String(params.student_id)
  }
  if (params?.status) {
    query.status = params.status
  }
  return mealRequest<{ items: DietNoteChangeRequest[], total: number }>('/diet-note-change-requests', 'GET', undefined, query)
}

export function reviewDietNoteChangeRequest(id: number, data: { status: 'approved' | 'rejected', review_note?: string }) {
  return mealRequest<DietNoteChangeRequest>(`/diet-note-change-requests/${id}/review`, 'POST', data)
}

export function upsertMeal(data: {
  meal_date: string
  menu_text: string
  photo_url?: string
  adjustment_note?: string
}) {
  return mealRequest<MealPlan>('/meals', 'POST', data)
}

export function copyMeal(data: { source_date: string, target_date: string }) {
  return mealRequest<MealPlan>('/meals/copy', 'POST', data)
}

export function uploadMealPhoto(filePath: string, formData: { meal_plan_id?: number } = {}) {
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
      url: `${appEnv.apiBaseUrl}/uploads/meals`,
      header: getAccessToken() ? { Authorization: `Bearer ${getAccessToken()}` } : undefined,
      success: (result) => {
        try {
          const response = JSON.parse(result.data) as ApiEnvelope<{ url: string }>
          if (result.statusCode >= 200 && result.statusCode < 300 && response.code === 0) {
            resolve(response.data.url)
            return
          }
          reject(new Error(response.message || '餐食照片上传失败'))
        }
        catch (error) {
          reject(error)
        }
      },
      fail: reject,
    })
  })
}

export function mealPhotoURL(path?: string) {
  return mediaURL(path)
}
