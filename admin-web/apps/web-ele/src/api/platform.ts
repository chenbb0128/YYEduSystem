import { businessRequestClient } from '#/api/request';

export namespace PlatformApi {
  export interface Overview {
    activeOrganizationCount: number;
    availableInviteCount: number;
    disabledOrganizationCount: number;
    exhaustedInviteCount: number;
    organizationCount: number;
    pendingRegistrationCount: number;
  }

  export interface Organization {
    authorizedUntil?: string;
    contactName: string;
    contactPhone: string;
    createdAt: string;
    id: number;
    name: string;
    slug: string;
    status: 'active' | 'disabled' | 'pending';
  }

  export interface Invite {
    codeHint: string;
    createdAt: string;
    expiresAt?: string;
    id: number;
    maxUses: number;
    note: string;
    status: 'active' | 'exhausted' | 'revoked';
    usedCount: number;
  }

  export interface Registration {
    adminUsername: string;
    contactName: string;
    contactPhone: string;
    createdAt: string;
    id: number;
    inviteId: number;
    organizationId?: number;
    organizationName: string;
    reviewNote: string;
    reviewedAt?: string;
    slug: string;
    status: 'approved' | 'pending' | 'rejected';
  }

  export interface PlatformAdmin {
    createdAt: string;
    id: number;
    realName: string;
    role: 'platform_admin';
    status: 'active' | 'disabled';
    username: string;
  }

  export interface ListResult<T> {
    items: T[];
    total: number;
  }
}

export function getPlatformOverviewApi() {
  return businessRequestClient.get<PlatformApi.Overview>('/platform/overview');
}

export function getPlatformOrganizationsApi(params?: {
  keyword?: string;
  status?: '' | PlatformApi.Organization['status'];
}) {
  return businessRequestClient.get<
    PlatformApi.ListResult<PlatformApi.Organization>
  >('/platform/organizations', { params });
}

export function setPlatformOrganizationStatusApi(
  id: number,
  status: PlatformApi.Organization['status'],
) {
  return businessRequestClient.post(`/platform/organizations/${id}/status`, {
    status,
  });
}

export function setPlatformOrganizationAuthorizationApi(
  id: number,
  authorizedUntil: string,
) {
  return businessRequestClient.put(
    `/platform/organizations/${id}/authorization`,
    { authorizedUntil },
  );
}

export function getPlatformInvitesApi(status = '') {
  return businessRequestClient.get<PlatformApi.ListResult<PlatformApi.Invite>>(
    '/platform/invites',
    { params: status ? { status } : undefined },
  );
}

export function createPlatformInviteApi(data: {
  expiresAt?: string;
  maxUses: number;
  note?: string;
}) {
  return businessRequestClient.post<{
    code: string;
    invite: PlatformApi.Invite;
    warning: string;
  }>('/platform/invites', data);
}

export function revokePlatformInviteApi(id: number) {
  return businessRequestClient.post(`/platform/invites/${id}/revoke`);
}

export function getPlatformRegistrationsApi(status = '') {
  return businessRequestClient.get<
    PlatformApi.ListResult<PlatformApi.Registration>
  >('/platform/registrations', { params: status ? { status } : undefined });
}

export function reviewPlatformRegistrationApi(
  id: number,
  data: { reviewNote?: string; status: 'approved' | 'rejected' },
) {
  return businessRequestClient.post(
    `/platform/registrations/${id}/review`,
    data,
  );
}

export function getPlatformAdminsApi(params?: {
  keyword?: string;
  status?: '' | 'active' | 'disabled';
}) {
  return businessRequestClient.get<
    PlatformApi.ListResult<PlatformApi.PlatformAdmin>
  >('/platform/admins', { params });
}

export function createPlatformAdminApi(data: {
  password: string;
  realName: string;
  status?: 'active' | 'disabled';
  username: string;
}) {
  return businessRequestClient.post<PlatformApi.PlatformAdmin>(
    '/platform/admins',
    data,
  );
}

export function updatePlatformAdminApi(
  id: number,
  data: {
    password?: string;
    realName: string;
    status: 'active' | 'disabled';
  },
) {
  return businessRequestClient.put<PlatformApi.PlatformAdmin>(
    `/platform/admins/${id}`,
    data,
  );
}

export function setPlatformAdminStatusApi(
  id: number,
  status: 'active' | 'disabled',
) {
  return businessRequestClient.post(`/platform/admins/${id}/status`, {
    status,
  });
}

export async function registerOrganizationApi(data: {
  adminPassword: string;
  adminUsername: string;
  contactName?: string;
  contactPhone?: string;
  inviteCode: string;
  organizationName: string;
  slug?: string;
}) {
  return businessRequestClient.post('/auth/organization-register', data);
}
