import { businessRequestClient } from '#/api/request';

export type SystemUserRole = 'admin' | 'editor' | 'teacher' | 'viewer';
export type SystemUserStatus = 'active' | 'disabled';

export interface SystemUserRecord {
  createdAt: string;
  id: number;
  realName: string;
  role: SystemUserRole;
  status: SystemUserStatus;
  username: string;
}

export interface SystemUserPayload {
  password?: string;
  realName: string;
  role: SystemUserRole;
  status: SystemUserStatus;
  username: string;
}

export interface SystemUserListParams {
  keyword?: string;
  page: number;
  pageSize: number;
  status?: SystemUserStatus;
}

export interface PageResult<T> {
  items: T[];
  total: number;
}

export function getSystemUsersApi(params: SystemUserListParams) {
  return businessRequestClient.get<PageResult<SystemUserRecord>>(
    '/system/users',
    {
      params,
    },
  );
}

export function createSystemUserApi(data: SystemUserPayload) {
  return businessRequestClient.post<SystemUserRecord>('/system/users', data);
}

export function updateSystemUserApi(id: number, data: SystemUserPayload) {
  return businessRequestClient.put<SystemUserRecord>(
    `/system/users/${id}`,
    data,
  );
}

export function deleteSystemUserApi(id: number) {
  return businessRequestClient.delete<boolean>(`/system/users/${id}`);
}
