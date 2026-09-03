-- name: ListPickupOperations :many
SELECT id, organization_id, operation_date, pickup_mode, school_id, school_class_id,
       care_class_id, teacher_user_id, teacher_name, status, started_at, finished_at,
       notes, created_at, updated_at, confirmed_at, confirmed_by_user_id, confirmed_by_name,
       executing_teacher_user_id, executing_teacher_name, teacher_role, expected_pickup_time
FROM pickup_operations
WHERE organization_id = ?
ORDER BY operation_date DESC, id DESC;

-- name: GetPickupOperationByID :one
SELECT id, organization_id, operation_date, pickup_mode, school_id, school_class_id,
       care_class_id, teacher_user_id, teacher_name, status, started_at, finished_at,
       notes, created_at, updated_at, confirmed_at, confirmed_by_user_id, confirmed_by_name,
       executing_teacher_user_id, executing_teacher_name, teacher_role, expected_pickup_time
FROM pickup_operations
WHERE id = ? AND organization_id = ?
LIMIT 1;

-- name: CreatePickupOperation :execresult
INSERT INTO pickup_operations (
    organization_id, operation_date, pickup_mode, school_id, school_class_id,
    care_class_id, teacher_user_id, teacher_name, status, expected_pickup_time, notes
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?);

-- name: UpdatePickupOperationStatus :execresult
UPDATE pickup_operations
SET status = ?, started_at = COALESCE(?, started_at), finished_at = COALESCE(?, finished_at)
WHERE id = ? AND organization_id = ?;

-- name: ConfirmPickupOperation :execresult
UPDATE pickup_operations
SET status = 'confirmed', confirmed_at = ?, confirmed_by_user_id = ?, confirmed_by_name = ?,
    executing_teacher_user_id = ?, executing_teacher_name = ?, teacher_role = ?,
    expected_pickup_time = ?, notes = ?, updated_at = CURRENT_TIMESTAMP(3)
WHERE id = ? AND organization_id = ? AND status = 'draft';


-- name: CreatePickupOperationStudent :execresult
INSERT INTO pickup_operation_students (organization_id, operation_id, student_id, status)
VALUES (?, ?, ?, 'planned');

-- name: AddPickupOperationStudent :execresult
INSERT INTO pickup_operation_students (organization_id, operation_id, student_id, status, note, is_temporary, profile_pending, pickup_mode)
VALUES (?, ?, ?, 'planned', ?, ?, ?, ?);

-- name: CompletePickupOperationStudentProfile :execresult
UPDATE pickup_operation_students
SET profile_pending = FALSE, updated_at = CURRENT_TIMESTAMP(3)
WHERE operation_id = ? AND student_id = ? AND organization_id = ?;

-- name: ListPickupOperationStudents :many
SELECT os.id AS operation_student_id, os.organization_id, os.operation_id, os.student_id,
       s.name AS student_name, os.status, os.photo_url, os.checked_at, os.note,
       os.created_at, os.updated_at, os.is_temporary, os.profile_pending, os.pickup_mode
FROM pickup_operation_students os
JOIN students s ON s.id = os.student_id
WHERE os.operation_id = ? AND os.organization_id = ?
ORDER BY s.name, os.student_id;

-- name: GetPickupOperationStudent :one
SELECT os.id AS operation_student_id, os.organization_id, os.operation_id, os.student_id,
       s.name AS student_name, os.status, os.photo_url, os.checked_at, os.note,
       os.created_at, os.updated_at, os.is_temporary, os.profile_pending, os.pickup_mode
FROM pickup_operation_students os
JOIN students s ON s.id = os.student_id
WHERE os.operation_id = ? AND os.student_id = ? AND os.organization_id = ?
LIMIT 1;

-- name: UpdatePickupOperationStudent :execresult
UPDATE pickup_operation_students
SET status = ?, photo_url = ?, checked_at = ?, note = ?
WHERE operation_id = ? AND student_id = ? AND organization_id = ?;

-- name: CreatePickupEvent :execresult
INSERT INTO pickup_events (
    organization_id, operation_id, operation_student_id, student_id,
    event_type, event_at, operator_name, photo_url, note
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListPickupEvents :many
SELECT id, organization_id, operation_id, operation_student_id, student_id,
       event_type, event_at, operator_name, photo_url, note
FROM pickup_events
WHERE operation_id = ? AND organization_id = ?
ORDER BY event_at DESC, id DESC;

-- name: GetPickupEventByID :one
SELECT id, organization_id, operation_id, operation_student_id, student_id,
       event_type, event_at, operator_name, photo_url, note
FROM pickup_events
WHERE id = ? AND operation_id = ? AND organization_id = ?
LIMIT 1;

-- name: CreateNotification :execresult
INSERT INTO notifications (
    organization_id, student_id, operation_id, event_id, recipient_type,
    kind, title, content, status
) VALUES (?, ?, ?, ?, 'parent', ?, ?, ?, 'pending');

-- name: ListNotifications :many
SELECT id, organization_id, student_id, operation_id, event_id, recipient_type,
       kind, title, content, status, sent_at, delivery_attempts, last_attempt_at,
       delivery_error, next_retry_at, created_at, read_at
FROM notifications
WHERE organization_id = ?
ORDER BY created_at DESC, id DESC;

-- name: UpdateNotificationStatus :execresult
UPDATE notifications
SET status = ?, sent_at = ?, delivery_attempts = ?, last_attempt_at = ?,
    delivery_error = ?, next_retry_at = ?
WHERE id = ? AND organization_id = ?;

-- name: MarkNotificationRead :execresult
UPDATE notifications
SET read_at = COALESCE(read_at, CURRENT_TIMESTAMP(3))
WHERE id = ? AND organization_id = ?;

-- name: CreatePickupChangeRequest :execresult
INSERT INTO pickup_change_requests (
    organization_id, student_id, operation_id, change_date, requested_status,
    note, submitted_by, status
) VALUES (?, ?, ?, ?, ?, ?, ?, 'pending');

-- name: ListPickupChangeRequests :many
SELECT r.id, r.organization_id, r.student_id, s.name AS student_name, r.operation_id,
       r.change_date, r.requested_status, r.note, r.submitted_by, r.status,
       r.reviewed_by_user_id, r.reviewed_at, r.review_note, r.created_at, r.updated_at
FROM pickup_change_requests r
JOIN students s ON s.id = r.student_id
WHERE r.organization_id = ?
ORDER BY r.change_date DESC, r.created_at DESC, r.id DESC;

-- name: GetPickupChangeRequest :one
SELECT r.id, r.organization_id, r.student_id, s.name AS student_name, r.operation_id,
       r.change_date, r.requested_status, r.note, r.submitted_by, r.status,
       r.reviewed_by_user_id, r.reviewed_at, r.review_note, r.created_at, r.updated_at
FROM pickup_change_requests r
JOIN students s ON s.id = r.student_id
WHERE r.id = ? AND r.organization_id = ?
LIMIT 1;

-- name: ReviewPickupChangeRequest :execresult
UPDATE pickup_change_requests
SET status = ?, reviewed_by_user_id = ?, reviewed_at = ?, review_note = ?, updated_at = CURRENT_TIMESTAMP(3)
WHERE id = ? AND organization_id = ? AND status = 'pending';
