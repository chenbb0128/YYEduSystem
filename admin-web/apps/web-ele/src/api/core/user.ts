import type { UserInfo } from '@vben/types';

import { businessRequestClient } from '#/api/request';

/**
 * 获取用户信息
 */
export async function getUserInfoApi() {
  return businessRequestClient.get<UserInfo>('/auth/me');
}
