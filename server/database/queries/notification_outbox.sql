-- name: GetNotificationByID :one
SELECT id, organization_id, student_id, operation_id, event_id, recipient_type,
       kind, title, content, status, sent_at, delivery_attempts, last_attempt_at,
       delivery_error, next_retry_at, read_at, created_at
FROM notifications
WHERE id = ? AND organization_id = ?
LIMIT 1;

-- name: CreateNotificationOutbox :execresult
INSERT INTO outbox_events (
    organization_id, event_type, aggregate_type, aggregate_id,
    notification_id, payload_json, status, available_at
) VALUES (?, 'notification.created', 'notification', ?, ?, JSON_OBJECT('notification_id', ?), 'pending', CURRENT_TIMESTAMP(3));

-- name: ListNotificationOutbox :many
SELECT id, organization_id, event_type, aggregate_type, aggregate_id,
       notification_id, status, attempts, available_at, locked_at, processed_at,
       last_error, created_at, updated_at
FROM outbox_events
WHERE status IN ('pending', 'failed')
  AND available_at <= ?
  AND (locked_at IS NULL OR locked_at <= ?)
ORDER BY id
LIMIT ?;

-- name: ClaimNotificationOutbox :execresult
UPDATE outbox_events
SET status = 'processing', attempts = attempts + 1, locked_at = ?, updated_at = CURRENT_TIMESTAMP(3)
WHERE id = ? AND organization_id = ?
  AND status IN ('pending', 'failed')
  AND (locked_at IS NULL OR locked_at <= ?);

-- name: CompleteNotificationOutbox :execresult
UPDATE outbox_events
SET status = ?, available_at = COALESCE(?, available_at), locked_at = NULL,
    processed_at = CASE WHEN ? = 'processed' THEN CURRENT_TIMESTAMP(3) ELSE processed_at END,
    last_error = ?, updated_at = CURRENT_TIMESTAMP(3)
WHERE id = ? AND organization_id = ? AND status = 'processing';

-- name: CreateNotificationDeliveryLog :execresult
INSERT INTO notification_delivery_logs (
    organization_id, notification_id, parent_account_id, message_kind, template_id, status
) VALUES (?, ?, ?, ?, ?, 'pending')
ON DUPLICATE KEY UPDATE id = LAST_INSERT_ID(id);

-- name: GetNotificationDeliveryLogByID :one
SELECT id, organization_id, notification_id, parent_account_id, message_kind,
       template_id, status, attempts, last_attempt_at, sent_at, next_retry_at,
       delivery_error, created_at, updated_at
FROM notification_delivery_logs
WHERE id = ? AND organization_id = ?
LIMIT 1;

-- name: ListNotificationDeliveryLogs :many
SELECT id, organization_id, notification_id, parent_account_id, message_kind,
       template_id, status, attempts, last_attempt_at, sent_at, next_retry_at,
       delivery_error, created_at, updated_at
FROM notification_delivery_logs
WHERE organization_id = ?
  AND (sqlc.narg('notification_id_filter') IS NULL OR notification_id = sqlc.narg('notification_id_filter'))
  AND (sqlc.narg('status_filter') IS NULL OR status = sqlc.narg('status_filter'))
ORDER BY id DESC;

-- name: UpdateNotificationDeliveryLogStatus :execresult
UPDATE notification_delivery_logs
SET status = ?, attempts = ?, last_attempt_at = ?, sent_at = ?, next_retry_at = ?,
    delivery_error = ?, updated_at = CURRENT_TIMESTAMP(3)
WHERE id = ? AND organization_id = ?;

-- name: RetryNotificationOutbox :execresult
UPDATE outbox_events
SET status = 'pending', attempts = 0, available_at = CURRENT_TIMESTAMP(3),
    locked_at = NULL, processed_at = NULL, last_error = '', updated_at = CURRENT_TIMESTAMP(3)
WHERE notification_id = ? AND organization_id = ? AND status = 'failed';
