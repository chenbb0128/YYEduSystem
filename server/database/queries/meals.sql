-- name: ListMealPlans :many
SELECT id, organization_id, meal_date, menu_text, photo_url, adjustment_note,
       created_by_user_id, created_by_name, status, created_at, updated_at
FROM meal_plans
WHERE organization_id = ?
  AND (sqlc.narg('from_date') IS NULL OR meal_date >= sqlc.narg('from_date'))
  AND (sqlc.narg('to_date') IS NULL OR meal_date <= sqlc.narg('to_date'))
ORDER BY meal_date DESC, id DESC;

-- name: GetMealPlanByID :one
SELECT id, organization_id, meal_date, menu_text, photo_url, adjustment_note,
       created_by_user_id, created_by_name, status, created_at, updated_at
FROM meal_plans WHERE organization_id = ? AND id = ? LIMIT 1;

-- name: GetMealPlanByDate :one
SELECT id, organization_id, meal_date, menu_text, photo_url, adjustment_note,
       created_by_user_id, created_by_name, status, created_at, updated_at
FROM meal_plans WHERE organization_id = ? AND meal_date = ? LIMIT 1;

-- name: CreateMealPlan :execresult
INSERT INTO meal_plans (organization_id, meal_date, menu_text, photo_url, adjustment_note, created_by_user_id, created_by_name, status)
VALUES (?, ?, ?, ?, ?, ?, ?, 'active');

-- name: UpdateMealPlan :execresult
UPDATE meal_plans
SET menu_text = ?, photo_url = ?, adjustment_note = ?, created_by_user_id = ?, created_by_name = ?, status = 'active', updated_at = CURRENT_TIMESTAMP(3)
WHERE organization_id = ? AND id = ?;

-- name: ListStudentDietNotes :many
SELECT id, organization_id, student_id, note, updated_by_user_id, updated_by_name, created_at, updated_at
FROM student_diet_notes
WHERE organization_id = ? AND (sqlc.narg('student_id_filter') IS NULL OR student_id = sqlc.narg('student_id_filter'))
ORDER BY student_id;

-- name: GetStudentDietNote :one
SELECT id, organization_id, student_id, note, updated_by_user_id, updated_by_name, created_at, updated_at
FROM student_diet_notes WHERE organization_id = ? AND student_id = ? LIMIT 1;

-- name: CreateStudentDietNote :execresult
INSERT INTO student_diet_notes (organization_id, student_id, note, updated_by_user_id, updated_by_name)
VALUES (?, ?, ?, ?, ?);

-- name: UpdateStudentDietNote :execresult
UPDATE student_diet_notes
SET note = ?, updated_by_user_id = ?, updated_by_name = ?, updated_at = CURRENT_TIMESTAMP(3)
WHERE organization_id = ? AND student_id = ?;

-- name: ListDietNoteChangeRequests :many
SELECT id, organization_id, student_id, parent_account_id, current_note, requested_note,
       status, review_note, reviewed_by_user_id, reviewed_at, created_at, updated_at
FROM diet_note_change_requests
WHERE organization_id = ?
  AND (sqlc.narg('student_id_filter') IS NULL OR student_id = sqlc.narg('student_id_filter'))
  AND (sqlc.narg('status_filter') IS NULL OR status = sqlc.narg('status_filter'))
ORDER BY CASE WHEN status = 'pending' THEN 0 ELSE 1 END, created_at DESC, id DESC;

-- name: GetDietNoteChangeRequest :one
SELECT id, organization_id, student_id, parent_account_id, current_note, requested_note,
       status, review_note, reviewed_by_user_id, reviewed_at, created_at, updated_at
FROM diet_note_change_requests
WHERE organization_id = ? AND id = ?
LIMIT 1;

-- name: GetPendingDietNoteChangeRequest :one
SELECT id, organization_id, student_id, parent_account_id, current_note, requested_note,
       status, review_note, reviewed_by_user_id, reviewed_at, created_at, updated_at
FROM diet_note_change_requests
WHERE organization_id = ? AND student_id = ? AND status = 'pending'
ORDER BY id DESC
LIMIT 1
FOR UPDATE;

-- name: CreateDietNoteChangeRequest :execresult
INSERT INTO diet_note_change_requests (
    organization_id, student_id, parent_account_id, current_note, requested_note, status
) VALUES (?, ?, ?, ?, ?, 'pending');

-- name: ReviewDietNoteChangeRequest :execresult
UPDATE diet_note_change_requests
SET status = ?, review_note = ?, reviewed_by_user_id = ?, reviewed_at = ?, updated_at = CURRENT_TIMESTAMP(3)
WHERE organization_id = ? AND id = ? AND status = 'pending';
