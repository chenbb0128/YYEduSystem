-- name: CreateParentAccount :execresult
INSERT INTO parent_accounts (organization_id, openid, nickname, avatar, status)
VALUES (?, ?, ?, ?, 'active');

-- name: GetParentAccountByID :one
SELECT id, organization_id, openid, nickname, avatar, status, created_at, updated_at
FROM parent_accounts
WHERE id = ? AND organization_id = ?
LIMIT 1;

-- name: GetParentAccountByOpenID :one
SELECT id, organization_id, openid, nickname, avatar, status, created_at, updated_at
FROM parent_accounts
WHERE openid = ? AND organization_id = ?
LIMIT 1;

-- name: CreateParentStudentBinding :execresult
INSERT INTO parent_student_bindings (
    organization_id, parent_account_id, student_id, relationship, is_primary, status
) VALUES (?, ?, ?, ?, ?, 'active');

-- name: ListParentAccountsForStudent :many
SELECT p.id, p.organization_id, p.openid, p.nickname, p.avatar, p.status,
       p.created_at, p.updated_at
FROM parent_accounts p
JOIN parent_student_bindings b ON b.parent_account_id = p.id
WHERE b.organization_id = ? AND b.student_id = ?
  AND b.status = 'active' AND p.status = 'active'
ORDER BY p.id;

-- name: ListParentStudentBindings :many
SELECT b.id AS binding_id, b.organization_id, b.parent_account_id, b.student_id,
       s.name AS student_name, s.school_class_id, s.care_class_id,
       b.relationship, b.is_primary, b.status, b.created_at, b.updated_at
FROM parent_student_bindings b
JOIN students s ON s.id = b.student_id
WHERE b.parent_account_id = ? AND b.organization_id = ? AND b.status = 'active'
ORDER BY s.name, b.student_id;

-- name: ListParentMessageSubscriptions :many
SELECT parent_account_id, message_kind, status, template_version, authorized_at, updated_at
FROM parent_message_subscriptions
WHERE organization_id = ? AND parent_account_id = ?
ORDER BY message_kind;

-- name: UpsertParentMessageSubscription :execresult
INSERT INTO parent_message_subscriptions (
    organization_id, parent_account_id, message_kind, status, template_version,
    authorized_at
) VALUES (?, ?, ?, sqlc.arg('status'), ?, CASE WHEN sqlc.arg('status') = 'accept' THEN CURRENT_TIMESTAMP(3) ELSE NULL END)
ON DUPLICATE KEY UPDATE
    status = VALUES(status),
    template_version = VALUES(template_version),
    authorized_at = CASE WHEN VALUES(status) = 'accept' THEN CURRENT_TIMESTAMP(3) ELSE authorized_at END,
    updated_at = CURRENT_TIMESTAMP(3);

-- name: CreateLeaveRequest :execresult
INSERT INTO leave_requests (
    organization_id, student_id, parent_account_id, submitted_by_type,
    submitted_by_user_id, leave_date, reason, status
) VALUES (?, ?, ?, ?, ?, ?, ?, 'pending');

-- name: CreateTeacherLeaveRequest :execresult
INSERT INTO leave_requests (
    organization_id, student_id, parent_account_id, submitted_by_type,
    submitted_by_user_id, leave_date, reason, status, teacher_note,
    reviewed_by_user_id, reviewed_at
) VALUES (?, ?, NULL, 'teacher', ?, ?, ?, 'approved', '老师口头代记', ?, CURRENT_TIMESTAMP(3));

-- name: FindActiveTeacherLeaveRequest :one
SELECT id, organization_id, student_id, parent_account_id, submitted_by_type,
       submitted_by_user_id, leave_date, reason, status, teacher_note,
       reviewed_by_user_id, reviewed_at, created_at, updated_at
FROM leave_requests
WHERE organization_id = ? AND student_id = ? AND leave_date = ?
  AND status IN ('pending', 'approved')
ORDER BY id DESC
LIMIT 1;

-- name: ListParentLeaveRequests :many
SELECT id, organization_id, student_id, parent_account_id, submitted_by_type,
       submitted_by_user_id, leave_date, reason, status, teacher_note,
       reviewed_by_user_id, reviewed_at, created_at, updated_at
FROM leave_requests
WHERE organization_id = ? AND parent_account_id = ?
ORDER BY leave_date DESC, id DESC;

-- name: ListAllLeaveRequests :many
SELECT id, organization_id, student_id, parent_account_id, submitted_by_type,
       submitted_by_user_id, leave_date, reason, status, teacher_note,
       reviewed_by_user_id, reviewed_at, created_at, updated_at
FROM leave_requests
WHERE organization_id = ?
ORDER BY leave_date DESC, id DESC;

-- name: ListApprovedLeaveStudentIDs :many
SELECT student_id
FROM leave_requests
WHERE organization_id = ? AND leave_date = ? AND status = 'approved';

-- name: GetLeaveRequestByID :one
SELECT id, organization_id, student_id, parent_account_id, submitted_by_type,
       submitted_by_user_id, leave_date, reason, status, teacher_note,
       reviewed_by_user_id, reviewed_at, created_at, updated_at
FROM leave_requests
WHERE id = ? AND organization_id = ?
LIMIT 1;

-- name: ReviewLeaveRequest :execresult
UPDATE leave_requests
SET status = ?, teacher_note = ?, reviewed_by_user_id = ?, reviewed_at = ?
WHERE id = ? AND organization_id = ? AND status = 'pending';

-- name: UpdateParentLeaveRequest :execresult
UPDATE leave_requests
SET leave_date = ?, reason = ?, updated_at = CURRENT_TIMESTAMP(3)
WHERE id = ? AND organization_id = ? AND parent_account_id = ? AND status = 'pending';

-- name: CancelParentLeaveRequest :execresult
UPDATE leave_requests
SET status = 'cancelled', updated_at = CURRENT_TIMESTAMP(3)
WHERE id = ? AND organization_id = ? AND parent_account_id = ? AND status = 'pending';

-- name: GetLatestParentPrivacyConsent :one
SELECT id, organization_id, parent_account_id, policy_version, consented_at, created_at
FROM parent_privacy_consents
WHERE organization_id = ? AND parent_account_id = ?
ORDER BY consented_at DESC, id DESC
LIMIT 1;

-- name: RecordParentPrivacyConsent :execresult
INSERT INTO parent_privacy_consents (
    organization_id, parent_account_id, policy_version, consented_at
) VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE policy_version = VALUES(policy_version);
