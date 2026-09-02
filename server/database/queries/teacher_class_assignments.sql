-- name: ListTeacherClassAssignments :many
SELECT a.id, a.organization_id, a.teacher_user_id, a.school_class_id,
       a.status, a.created_at, a.updated_at
FROM teacher_class_assignments a
WHERE a.organization_id = ?
  AND (sqlc.arg(teacher_user_id) = 0 OR a.teacher_user_id = sqlc.arg(teacher_user_id))
  AND (sqlc.arg(school_class_id) = 0 OR a.school_class_id = sqlc.arg(school_class_id))
ORDER BY a.status, a.id DESC;

-- name: GetTeacherClassAssignment :one
SELECT id, organization_id, teacher_user_id, school_class_id,
       status, created_at, updated_at
FROM teacher_class_assignments
WHERE id = ? AND organization_id = ?
LIMIT 1;

-- name: GetTeacherClassAssignmentByPair :one
SELECT id, organization_id, teacher_user_id, school_class_id,
       status, created_at, updated_at
FROM teacher_class_assignments
WHERE organization_id = ? AND teacher_user_id = ? AND school_class_id = ?
LIMIT 1;

-- name: CreateTeacherClassAssignment :execresult
INSERT INTO teacher_class_assignments (
    organization_id, teacher_user_id, school_class_id, status
) VALUES (?, ?, ?, 'active');

-- name: SetTeacherClassAssignmentStatus :execresult
UPDATE teacher_class_assignments
SET status = ?
WHERE id = ? AND organization_id = ?;
