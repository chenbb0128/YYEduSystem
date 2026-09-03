-- name: ListOrganizations :many
SELECT id, name, slug, contact_name, contact_phone, authorized_until, status, created_at, updated_at
FROM organizations
ORDER BY id ASC;

-- name: CreateOrganization :execresult
INSERT INTO organizations (name, slug, contact_name, contact_phone, authorized_until, status)
VALUES (?, ?, ?, ?, ?, ?);

-- name: SetOrganizationStatus :execresult
UPDATE organizations SET status = ? WHERE id = ?;

-- name: CreateOrganizationInvite :execresult
INSERT INTO organization_invites (code_hash, code_hint, max_uses, expires_at, status, note, created_by_user_id)
VALUES (?, ?, ?, ?, 'active', ?, ?);

-- name: GetOrganizationInviteByHash :one
SELECT id, code_hash, code_hint, max_uses, used_count, expires_at, status, note, created_by_user_id, created_at, updated_at
FROM organization_invites
WHERE code_hash = ?
LIMIT 1;

-- name: ListOrganizationInvites :many
SELECT id, code_hash, code_hint, max_uses, used_count, expires_at, status, note, created_by_user_id, created_at, updated_at
FROM organization_invites
WHERE (sqlc.narg('status_filter') IS NULL OR status = sqlc.narg('status_filter'))
ORDER BY id DESC;

-- name: ConsumeOrganizationInvite :execresult
UPDATE organization_invites
SET used_count = used_count + 1,
    status = CASE WHEN used_count + 1 >= max_uses THEN 'exhausted' ELSE 'active' END
WHERE id = ? AND status = 'active' AND used_count < max_uses
  AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP(3));

-- name: RevokeOrganizationInvite :execresult
UPDATE organization_invites SET status = 'revoked' WHERE id = ? AND status = 'active';

-- name: CreateOrganizationRegistration :execresult
INSERT INTO organization_registrations (invite_id, organization_name, slug, contact_name, contact_phone, admin_username, admin_password_hash, status)
VALUES (?, ?, ?, ?, ?, ?, ?, 'pending');

-- name: GetOrganizationRegistration :one
SELECT id, invite_id, organization_id, organization_name, slug, contact_name, contact_phone, admin_username, admin_password_hash, status, review_note, reviewed_by_user_id, reviewed_at, created_at, updated_at
FROM organization_registrations
WHERE id = ?
LIMIT 1;

-- name: ListOrganizationRegistrations :many
SELECT id, invite_id, organization_id, organization_name, slug, contact_name, contact_phone, admin_username, admin_password_hash, status, review_note, reviewed_by_user_id, reviewed_at, created_at, updated_at
FROM organization_registrations
WHERE (sqlc.narg('status_filter') IS NULL OR status = sqlc.narg('status_filter'))
ORDER BY id DESC;

-- name: SetOrganizationRegistrationStatus :execresult
UPDATE organization_registrations
SET status = ?, organization_id = ?, review_note = ?, reviewed_by_user_id = ?, reviewed_at = CURRENT_TIMESTAMP(3)
WHERE id = ? AND status = 'pending';
