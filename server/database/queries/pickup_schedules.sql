-- name: ListPickupSchedules :many
SELECT id, organization_id, school_id, school_class_id, care_class_id,
       weekday, pickup_mode, teacher_user_id, teacher_name, expected_pickup_time,
       effective_from, effective_to, enabled, notes, created_at, updated_at
FROM pickup_schedules
WHERE organization_id = ?
ORDER BY enabled DESC, weekday ASC, expected_pickup_time ASC, id DESC;

-- name: GetPickupScheduleByID :one
SELECT id, organization_id, school_id, school_class_id, care_class_id,
       weekday, pickup_mode, teacher_user_id, teacher_name, expected_pickup_time,
       effective_from, effective_to, enabled, notes, created_at, updated_at
FROM pickup_schedules
WHERE id = ? AND organization_id = ?
LIMIT 1;

-- name: CreatePickupSchedule :execresult
INSERT INTO pickup_schedules (
    organization_id, school_id, school_class_id, care_class_id, weekday,
    pickup_mode, teacher_user_id, teacher_name, expected_pickup_time,
    effective_from, effective_to, enabled, notes
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdatePickupSchedule :execresult
UPDATE pickup_schedules
SET school_id = ?, school_class_id = ?, care_class_id = ?, weekday = ?,
    pickup_mode = ?, teacher_user_id = ?, teacher_name = ?, expected_pickup_time = ?,
    effective_from = ?, effective_to = ?, enabled = ?, notes = ?,
    updated_at = CURRENT_TIMESTAMP(3)
WHERE id = ? AND organization_id = ?;
