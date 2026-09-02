-- +goose Up
ALTER TABLE pickup_operations
    ADD COLUMN confirmed_at DATETIME(3) NULL AFTER finished_at,
    ADD COLUMN confirmed_by_user_id BIGINT UNSIGNED NULL AFTER confirmed_at,
    ADD COLUMN confirmed_by_name VARCHAR(64) NOT NULL DEFAULT '' AFTER confirmed_by_user_id,
    ADD COLUMN executing_teacher_user_id BIGINT UNSIGNED NULL AFTER confirmed_by_name,
    ADD COLUMN executing_teacher_name VARCHAR(64) NOT NULL DEFAULT '' AFTER executing_teacher_user_id,
    ADD COLUMN teacher_role VARCHAR(32) NOT NULL DEFAULT 'lead' AFTER executing_teacher_name,
    ADD COLUMN expected_pickup_time VARCHAR(16) NOT NULL DEFAULT '' AFTER teacher_role,
    ADD KEY idx_pickup_operations_confirmed (organization_id, operation_date, confirmed_at),
    ADD CONSTRAINT fk_pickup_operations_confirmed_by FOREIGN KEY (confirmed_by_user_id) REFERENCES users (id),
    ADD CONSTRAINT fk_pickup_operations_executing_teacher FOREIGN KEY (executing_teacher_user_id) REFERENCES users (id);

ALTER TABLE pickup_operations
    DROP CHECK chk_pickup_operations_status,
    ADD CONSTRAINT chk_pickup_operations_status CHECK (status IN ('draft', 'confirmed', 'started', 'finished', 'cancelled'));

ALTER TABLE pickup_operation_students
    ADD COLUMN is_temporary BOOLEAN NOT NULL DEFAULT FALSE AFTER note,
    ADD COLUMN profile_pending BOOLEAN NOT NULL DEFAULT FALSE AFTER is_temporary,
    ADD COLUMN pickup_mode VARCHAR(32) NOT NULL DEFAULT '' AFTER profile_pending,
    ADD CONSTRAINT chk_pickup_operation_students_mode CHECK (pickup_mode IN ('', 'school_pickup', 'self_arrival', 'parent_picked_up'));

ALTER TABLE notifications
    ADD COLUMN read_at DATETIME(3) NULL AFTER sent_at,
    ADD KEY idx_notifications_unread (organization_id, read_at, created_at);

CREATE TABLE pickup_change_requests (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    student_id BIGINT UNSIGNED NOT NULL,
    operation_id BIGINT UNSIGNED NULL,
    change_date DATE NOT NULL,
    requested_status VARCHAR(32) NOT NULL,
    note VARCHAR(500) NOT NULL DEFAULT '',
    submitted_by VARCHAR(32) NOT NULL DEFAULT 'parent',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    reviewed_by_user_id BIGINT UNSIGNED NULL,
    reviewed_at DATETIME(3) NULL,
    review_note VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_pickup_change_requests_day_status (organization_id, change_date, status),
    KEY idx_pickup_change_requests_student (organization_id, student_id, change_date),
    CONSTRAINT fk_pickup_change_requests_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_pickup_change_requests_student FOREIGN KEY (student_id) REFERENCES students (id),
    CONSTRAINT fk_pickup_change_requests_operation FOREIGN KEY (operation_id) REFERENCES pickup_operations (id),
    CONSTRAINT fk_pickup_change_requests_reviewer FOREIGN KEY (reviewed_by_user_id) REFERENCES users (id),
    CONSTRAINT chk_pickup_change_requests_status CHECK (status IN ('pending', 'approved', 'rejected')),
    CONSTRAINT chk_pickup_change_requests_requested_status CHECK (requested_status IN ('parent_picked_up', 'self_arrived', 'leave', 'absent'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE pickup_change_requests;
ALTER TABLE notifications
    DROP KEY idx_notifications_unread,
    DROP COLUMN read_at;
ALTER TABLE pickup_operation_students
    DROP CHECK chk_pickup_operation_students_mode,
    DROP COLUMN pickup_mode,
    DROP COLUMN profile_pending,
    DROP COLUMN is_temporary;
ALTER TABLE pickup_operations
    DROP CHECK chk_pickup_operations_status,
    ADD CONSTRAINT chk_pickup_operations_status CHECK (status IN ('draft', 'started', 'finished', 'cancelled')),
    DROP FOREIGN KEY fk_pickup_operations_executing_teacher,
    DROP FOREIGN KEY fk_pickup_operations_confirmed_by,
    DROP KEY idx_pickup_operations_confirmed,
    DROP COLUMN expected_pickup_time,
    DROP COLUMN teacher_role,
    DROP COLUMN executing_teacher_name,
    DROP COLUMN executing_teacher_user_id,
    DROP COLUMN confirmed_by_name,
    DROP COLUMN confirmed_by_user_id,
    DROP COLUMN confirmed_at;
