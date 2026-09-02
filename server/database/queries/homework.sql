-- name: ListHomeworkTasks :many
SELECT id, organization_id, homework_date, school_id, school_class_id,
       subject, content, attachment_urls, created_by_user_id, creator_name, status,
       created_at, updated_at
FROM homework_tasks
WHERE organization_id = ?
ORDER BY homework_date DESC, id DESC;

-- name: GetHomeworkTask :one
SELECT id, organization_id, homework_date, school_id, school_class_id,
       subject, content, attachment_urls, created_by_user_id, creator_name, status,
       created_at, updated_at
FROM homework_tasks
WHERE id = ? AND organization_id = ?
LIMIT 1;

-- name: CreateHomeworkTask :execresult
INSERT INTO homework_tasks (
    organization_id, homework_date, school_id, school_class_id,
    subject, content, attachment_urls, created_by_user_id, creator_name, status
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active');

-- name: CreateHomeworkTaskStudent :execresult
INSERT INTO homework_task_students (organization_id, task_id, student_id, status)
VALUES (?, ?, ?, 'pending');

-- name: ListHomeworkTaskStudents :many
SELECT hs.id AS homework_task_student_id, hs.organization_id, hs.task_id,
       hs.student_id, s.name AS student_name, hs.status, hs.correction_note,
       hs.reviewed_by_user_id, hs.reviewed_at, hs.created_at, hs.updated_at
FROM homework_task_students hs
JOIN students s ON s.id = hs.student_id
WHERE hs.task_id = ? AND hs.organization_id = ?
ORDER BY s.name, hs.student_id;

-- name: GetHomeworkTaskStudent :one
SELECT hs.id AS homework_task_student_id, hs.organization_id, hs.task_id,
       hs.student_id, s.name AS student_name, hs.status, hs.correction_note,
       hs.reviewed_by_user_id, hs.reviewed_at, hs.created_at, hs.updated_at
FROM homework_task_students hs
JOIN students s ON s.id = hs.student_id
WHERE hs.task_id = ? AND hs.student_id = ? AND hs.organization_id = ?
LIMIT 1;

-- name: UpdateHomeworkTaskStudentReview :execresult
UPDATE homework_task_students
SET status = ?, correction_note = ?, reviewed_by_user_id = ?, reviewed_at = ?
WHERE task_id = ? AND student_id = ? AND organization_id = ?;

-- name: ListStudentHomework :many
SELECT t.id AS task_id, t.organization_id, t.homework_date, t.school_id,
       t.school_class_id, t.subject, t.content, t.attachment_urls, t.created_by_user_id,
       t.creator_name, t.status AS task_status, t.created_at AS task_created_at,
       t.updated_at AS task_updated_at, hs.id AS homework_task_student_id,
       hs.student_id, s.name AS student_name, hs.status AS student_status,
       hs.correction_note, hs.reviewed_by_user_id, hs.reviewed_at,
       hs.created_at AS student_created_at, hs.updated_at AS student_updated_at
FROM homework_task_students hs
JOIN homework_tasks t ON t.id = hs.task_id
JOIN students s ON s.id = hs.student_id
WHERE hs.organization_id = ? AND hs.student_id = ? AND t.status = 'active'
ORDER BY t.homework_date DESC, t.id DESC;
