import type { AuthTokenResult } from '@/services/auth'
import { request } from '@/services/request'

export interface ParentOrganization {
  id: number
  name: string
  slug?: string
  status: string
  current: boolean
}

interface ApiEnvelope<T> {
  code: number
  data: T
  message: string
}

interface OrganizationList {
  items: ParentOrganization[]
  total: number
}

export function getParentOrganizations() {
  return request<ApiEnvelope<OrganizationList>>({ method: 'GET', url: '/parent/organizations' }).then((response) => {
    if (response.code !== 0) {
      throw new Error(response.message || '机构列表加载失败')
    }
    return response.data
  })
}

export function switchParentOrganization(input: { organization_id?: number, invite_token?: string }) {
  return request<ApiEnvelope<AuthTokenResult>>({ method: 'POST', url: '/parent/organizations/switch', data: input }).then((response) => {
    if (response.code !== 0) {
      throw new Error(response.message || '机构切换失败')
    }
    return response.data
  })
}
