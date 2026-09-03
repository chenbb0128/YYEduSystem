import { businessRequestClient } from '#/api/request';

export namespace PlatformApi {
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

  export interface ListResult<T> {
    items: T[];
    total: number;
  }
}

export function getPlatformOrganizationsApi() {
  return businessRequestClient.get<
    PlatformApi.ListResult<PlatformApi.Organization>
  >('/platform/organizations');
}

export function setPlatformOrganizationStatusApi(
  id: number,
  status: PlatformApi.Organization['status'],
) {
  return businessRequestClient.post(`/platform/organizations/${id}/status`, {
    status,
  });
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
