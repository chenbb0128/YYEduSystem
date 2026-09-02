-- name: CreateParentChildApplication :execresult
INSERT INTO parent_child_applications (
    organization_id, parent_account_id, student_name,
    school_name_input, grade_input, class_name_input,
    school_id, school_class_id, grade, class_name,
    guardian_name, guardian_phone, relationship, notes, status
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending');

-- name: GetParentChildApplication :one
SELECT id, organization_id, parent_account_id, student_id,
       student_name, school_name_input, grade_input, class_name_input,
       school_id, school_class_id, grade, class_name,
       guardian_name, guardian_phone, relationship, notes,
       status, review_note, reviewed_by_user_id, reviewed_at,
       created_at, updated_at
FROM parent_child_applications
WHERE id = ? AND organization_id = ?
LIMIT 1;

-- name: ListParentChildApplications :many
SELECT id, organization_id, parent_account_id, student_id,
       student_name, school_name_input, grade_input, class_name_input,
       school_id, school_class_id, grade, class_name,
       guardian_name, guardian_phone, relationship, notes,
       status, review_note, reviewed_by_user_id, reviewed_at,
       created_at, updated_at
FROM parent_child_applications
WHERE organization_id = ? AND parent_account_id = ?
ORDER BY created_at DESC, id DESC;

-- name: ListAllParentChildApplications :many
SELECT id, organization_id, parent_account_id, student_id,
       student_name, school_name_input, grade_input, class_name_input,
       school_id, school_class_id, grade, class_name,
       guardian_name, guardian_phone, relationship, notes,
       status, review_note, reviewed_by_user_id, reviewed_at,
       created_at, updated_at
FROM parent_child_applications
WHERE organization_id = ?
ORDER BY created_at DESC, id DESC;

-- name: UpdateParentChildApplication :execresult
UPDATE parent_child_applications
SET student_name = ?, school_name_input = ?, grade_input = ?, class_name_input = ?,
    school_id = ?, school_class_id = ?, grade = ?, class_name = ?,
    guardian_name = ?, guardian_phone = ?, relationship = ?, notes = ?,
    student_id = NULL, status = 'pending', review_note = '',
    reviewed_by_user_id = NULL, reviewed_at = NULL, updated_at = CURRENT_TIMESTAMP(3)
WHERE id = ? AND organization_id = ? AND parent_account_id = ? AND status = 'needs_info';

-- name: ReviewParentChildApplication :execresult
UPDATE parent_child_applications
SET status = ?, student_id = ?, school_id = ?, school_class_id = ?,
    review_note = ?, reviewed_by_user_id = ?, reviewed_at = ?
WHERE id = ? AND organization_id = ? AND status IN ('pending', 'needs_info');
