-- name: ListDailySummaries :many
SELECT id, organization_id, summary_date, content, child_updates_json, status, current_version,
       created_by_user_id, created_by_name, generated_at, published_at, closed_at,
       withdrawn_at, withdrawal_reason, correction_reason, created_at, updated_at
FROM daily_summaries
WHERE organization_id = ? AND (sqlc.narg('summary_date_filter') IS NULL OR summary_date = sqlc.narg('summary_date_filter'))
ORDER BY summary_date DESC, id DESC;

-- name: GetDailySummaryByID :one
SELECT id, organization_id, summary_date, content, child_updates_json, status, current_version,
       created_by_user_id, created_by_name, generated_at, published_at, closed_at,
       withdrawn_at, withdrawal_reason, correction_reason, created_at, updated_at
FROM daily_summaries WHERE organization_id = ? AND id = ? LIMIT 1;

-- name: GetDailySummaryByDate :one
SELECT id, organization_id, summary_date, content, child_updates_json, status, current_version,
       created_by_user_id, created_by_name, generated_at, published_at, closed_at,
       withdrawn_at, withdrawal_reason, correction_reason, created_at, updated_at
FROM daily_summaries WHERE organization_id = ? AND summary_date = ? LIMIT 1;

-- name: CreateDailySummary :execresult
INSERT INTO daily_summaries (organization_id, summary_date, content, child_updates_json, status, created_by_user_id, created_by_name, generated_at)
VALUES (?, ?, ?, ?, 'draft', ?, ?, ?);

-- name: UpdateDailySummary :execresult
UPDATE daily_summaries
SET content = ?, child_updates_json = ?, current_version = current_version + 1, updated_at = CURRENT_TIMESTAMP(3)
WHERE organization_id = ? AND id = ? AND status = 'draft';

-- name: PublishDailySummary :execresult
UPDATE daily_summaries
SET status = 'published', published_at = ?, updated_at = CURRENT_TIMESTAMP(3)
WHERE organization_id = ? AND id = ? AND status = 'draft';

-- name: CloseDailySummary :execresult
UPDATE daily_summaries
SET status = 'closed', closed_at = ?, updated_at = CURRENT_TIMESTAMP(3)
WHERE organization_id = ? AND id = ? AND status IN ('draft', 'published');

-- name: WithdrawDailySummary :execresult
UPDATE daily_summaries
SET status = 'withdrawn', withdrawn_at = ?, withdrawal_reason = ?, updated_at = CURRENT_TIMESTAMP(3)
WHERE organization_id = ? AND id = ? AND status = 'published';

-- name: CorrectDailySummary :execresult
UPDATE daily_summaries
SET content = ?, child_updates_json = ?, current_version = current_version + 1,
    status = 'published', published_at = ?, closed_at = NULL, withdrawn_at = NULL,
    withdrawal_reason = '', correction_reason = ?,
    created_by_user_id = ?, created_by_name = ?, updated_at = CURRENT_TIMESTAMP(3)
WHERE organization_id = ? AND id = ? AND status IN ('published', 'closed', 'withdrawn');

-- name: CreateDailySummaryVersion :exec
INSERT INTO daily_summary_versions (
    organization_id, daily_summary_id, version, action, content, child_updates_json,
    reason, created_by_user_id, created_by_name, created_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListDailySummaryVersions :many
SELECT id, organization_id, daily_summary_id, version, action, content, child_updates_json,
       reason, created_by_user_id, created_by_name, created_at
FROM daily_summary_versions
WHERE organization_id = ? AND daily_summary_id = ?
ORDER BY version DESC, id DESC;

-- name: MarkDailySummaryRead :exec
INSERT INTO daily_summary_reads (organization_id, daily_summary_id, parent_account_id, read_version, read_at)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE read_version = VALUES(read_version), read_at = VALUES(read_at);

-- name: GetDailySummaryReadAt :one
SELECT r.read_version, r.read_at, s.current_version
FROM daily_summary_reads r
JOIN daily_summaries s ON s.id = r.daily_summary_id AND s.organization_id = r.organization_id
WHERE r.organization_id = ? AND r.daily_summary_id = ? AND r.parent_account_id = ?
LIMIT 1;
