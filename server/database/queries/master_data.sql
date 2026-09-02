-- name: ListSchools :many
SELECT id, organization_id, name, address, contact_phone, status, created_at, updated_at
FROM schools
WHERE organization_id = ?
ORDER BY name, id;

-- name: CreateSchool :execresult
INSERT INTO schools (organization_id, name, address, contact_phone, status)
VALUES (?, ?, ?, ?, ?);

-- name: GetSchoolByID :one
SELECT id, organization_id, name, address, contact_phone, status, created_at, updated_at
FROM schools
WHERE id = ? AND organization_id = ?
LIMIT 1;

-- name: ListAcademicTerms :many
SELECT id, organization_id, name, starts_on, ends_on, is_current, status, created_at, updated_at
FROM academic_terms
WHERE organization_id = ?
ORDER BY starts_on DESC, id DESC;

-- name: CreateAcademicTerm :execresult
INSERT INTO academic_terms (organization_id, name, starts_on, ends_on, is_current, status)
VALUES (?, ?, ?, ?, ?, ?);

-- name: UnsetCurrentAcademicTerms :exec
UPDATE academic_terms
SET is_current = FALSE
WHERE organization_id = ? AND is_current = TRUE;

-- name: ListSchoolClasses :many
SELECT id, organization_id, school_id, term_id, grade, name, status, created_at, updated_at
FROM school_classes
WHERE organization_id = ?
ORDER BY school_id, term_id, grade, name, id;

-- name: CreateSchoolClass :execresult
INSERT INTO school_classes (organization_id, school_id, term_id, grade, name, status)
VALUES (?, ?, ?, ?, ?, ?);

-- name: ListCareClasses :many
SELECT id, organization_id, name, capacity, status, created_at, updated_at
FROM care_classes
WHERE organization_id = ?
ORDER BY name, id;

-- name: CreateCareClass :execresult
INSERT INTO care_classes (organization_id, name, capacity, status)
VALUES (?, ?, ?, ?);

-- name: ListStudents :many
SELECT id, organization_id, school_id, term_id, school_class_id, care_class_id,
       name, gender, birth_date, student_no, guardian_phone, emergency_contact,
       emergency_phone, status, notes, created_at, updated_at
FROM students
WHERE organization_id = ?
ORDER BY status, name, id;

-- name: CreateStudent :execresult
INSERT INTO students (
    organization_id, school_id, term_id, school_class_id, care_class_id,
    name, gender, birth_date, student_no, guardian_phone, emergency_contact,
    emergency_phone, status, notes
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetStudentByID :one
SELECT id, organization_id, school_id, term_id, school_class_id, care_class_id,
       name, gender, birth_date, student_no, guardian_phone, emergency_contact,
       emergency_phone, status, notes, created_at, updated_at
FROM students
WHERE id = ? AND organization_id = ?
LIMIT 1;

-- name: UpdateStudent :execresult
UPDATE students
SET school_id = ?, term_id = ?, school_class_id = ?, care_class_id = ?,
    name = ?, gender = ?, birth_date = ?, student_no = ?, guardian_phone = ?,
    emergency_contact = ?, emergency_phone = ?, status = ?, notes = ?
WHERE id = ? AND organization_id = ?;
