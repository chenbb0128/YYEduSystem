import { businessRequestClient } from '#/api/request';

export namespace AuthApi {
  /** 登录接口参数 */
  export interface LoginParams {
    password?: string;
    username?: string;
  }

  /** 登录接口返回值 */
  export interface LoginResult {
    accessToken: string;
    expiresIn?: number;
    principal?: string;
    refreshToken?: string;
    role?: string;
  }

  export interface RefreshTokenResult {
    accessToken: string;
    refreshToken?: string;
    expiresIn?: number;
  }
}

/**
 * 登录
 */
export async function loginApi(data: AuthApi.LoginParams) {
  return businessRequestClient.post<AuthApi.LoginResult>('/auth/login', data);
}

/**
 * 刷新accessToken
 */
const refreshTokenStorageKey = 'tuoguan.auth.refresh-token';

export async function refreshTokenApi(data?: { refreshToken?: string }) {
  const refreshToken =
    data?.refreshToken ||
    (typeof localStorage === 'undefined'
      ? ''
      : localStorage.getItem(refreshTokenStorageKey) || '');
  return businessRequestClient.post<AuthApi.RefreshTokenResult>(
    '/auth/refresh',
    {
      refreshToken,
    },
  );
}

export function saveRefreshToken(token?: string) {
  if (typeof localStorage === 'undefined') return;
  if (token) localStorage.setItem(refreshTokenStorageKey, token);
  else localStorage.removeItem(refreshTokenStorageKey);
}

/**
 * 退出登录
 */
export async function logoutApi() {
  return businessRequestClient.post('/auth/logout');
}

/**
 * 获取用户权限码
 */
export async function getAccessCodesApi() {
  return businessRequestClient.get<string[]>('/auth/codes');
}
