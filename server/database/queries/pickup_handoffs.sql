-- name: HandoffPickupOperation :execresult
UPDATE pickup_operations
SET executing_teacher_user_id = ?, executing_teacher_name = ?, teacher_role = ?, updated_at = CURRENT_TIMESTAMP(3)
WHERE id = ? AND organization_id = ? AND status = 'started';

-- name: CreatePickupHandoff :execresult
INSERT INTO pickup_operation_handoffs (
    organization_id, operation_id, from_teacher_user_id, from_teacher_name,
    to_teacher_user_id, to_teacher_name, teacher_role, note,
    handoff_at, created_by_user_id, created_by_name
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListPickupHandoffs :many
SELECT id, organization_id, operation_id, from_teacher_user_id, from_teacher_name,
       to_teacher_user_id, to_teacher_name, teacher_role, note, handoff_at,
       created_by_user_id, created_by_name
FROM pickup_operation_handoffs
WHERE organization_id = ? AND operation_id = ?
ORDER BY handoff_at DESC, id DESC;
