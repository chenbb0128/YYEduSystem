import { businessRequestClient } from '#/api/request';

export type DeliveryStatus = 'failed' | 'pending' | 'sent' | 'skipped';
export type NotificationMessageKind =
  | 'homework'
  | 'leave'
  | 'meal'
  | 'pickup'
  | 'summary';

export interface NotificationDeliveryLogRecord {
  id: number;
  notification_id: number;
  student_id: number;
  student_name: string;
  parent_account_id: number;
  message_kind: string;
  template_id: string;
  notification_status: string;
  notification_title: string;
  status: DeliveryStatus;
  attempts: number;
  last_attempt_at?: string;
  sent_at?: string;
  next_retry_at?: string;
  delivery_error?: string;
  created_at: string;
  updated_at: string;
}

interface PageResult<T> {
  items: T[];
  total: number;
}

export function getNotificationDeliveryLogsApi(params?: {
  message_kind?: NotificationMessageKind;
  notification_id?: number;
  status?: DeliveryStatus;
}) {
  return businessRequestClient.get<PageResult<NotificationDeliveryLogRecord>>(
    '/notifications/delivery-logs',
    { params },
  );
}

export function retryNotificationApi(notificationId: number) {
  return businessRequestClient.post<{ id: number; status: string }>(
    `/notifications/${notificationId}/retry`,
  );
}
