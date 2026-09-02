import { businessRequestClient } from '#/api/request';

export interface AuditLogRecord {
  id: number;
  actor_type: 'anonymous' | 'parent' | 'staff' | 'system';
  actor_id?: number;
  action: string;
  resource_type: string;
  resource_id?: number;
  metadata: unknown;
  request_id?: string;
  created_at: string;
}

interface PageResult<T> {
  items: T[];
  total: number;
}

export function getAuditLogsApi(params?: {
  action?: string;
  limit?: number;
  resource_type?: string;
}) {
  return businessRequestClient.get<PageResult<AuditLogRecord>>('/audit-logs', {
    params,
  });
}
