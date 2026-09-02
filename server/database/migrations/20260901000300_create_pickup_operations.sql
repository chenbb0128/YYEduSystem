-- +goose Up
CREATE TABLE pickup_operations (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    operation_date DATE NOT NULL,
    pickup_mode VARCHAR(32) NOT NULL DEFAULT 'school_pickup',
    school_id BIGINT UNSIGNED NOT NULL,
    school_class_id BIGINT UNSIGNED NOT NULL,
    care_class_id BIGINT UNSIGNED NULL,
    teacher_user_id BIGINT UNSIGNED NULL,
    teacher_name VARCHAR(64) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    started_at DATETIME(3) NULL,
    finished_at DATETIME(3) NULL,
    notes VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_pickup_operations_day_class (organization_id, operation_date, school_class_id),
    KEY idx_pickup_operations_day_status (organization_id, operation_date, status),
    CONSTRAINT fk_pickup_operations_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_pickup_operations_school FOREIGN KEY (school_id) REFERENCES schools (id),
    CONSTRAINT fk_pickup_operations_school_class FOREIGN KEY (school_class_id) REFERENCES school_classes (id),
    CONSTRAINT fk_pickup_operations_care_class FOREIGN KEY (care_class_id) REFERENCES care_classes (id),
    CONSTRAINT fk_pickup_operations_teacher FOREIGN KEY (teacher_user_id) REFERENCES users (id),
    CONSTRAINT chk_pickup_operations_mode CHECK (pickup_mode IN ('school_pickup', 'self_arrival')),
    CONSTRAINT chk_pickup_operations_status CHECK (status IN ('draft', 'started', 'finished', 'cancelled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE pickup_operation_students (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    operation_id BIGINT UNSIGNED NOT NULL,
    student_id BIGINT UNSIGNED NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'planned',
    photo_url VARCHAR(512) NOT NULL DEFAULT '',
    checked_at DATETIME(3) NULL,
    note VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_pickup_operation_students_member (operation_id, student_id),
    KEY idx_pickup_operation_students_status (operation_id, status),
    CONSTRAINT fk_pickup_operation_students_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_pickup_operation_students_operation FOREIGN KEY (operation_id) REFERENCES pickup_operations (id),
    CONSTRAINT fk_pickup_operation_students_student FOREIGN KEY (student_id) REFERENCES students (id),
    CONSTRAINT chk_pickup_operation_students_status CHECK (status IN ('planned', 'picked_up', 'self_arrived', 'parent_picked_up', 'leave', 'absent'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE pickup_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    operation_id BIGINT UNSIGNED NOT NULL,
    operation_student_id BIGINT UNSIGNED NOT NULL,
    student_id BIGINT UNSIGNED NOT NULL,
    event_type VARCHAR(32) NOT NULL,
    event_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    operator_name VARCHAR(64) NOT NULL DEFAULT '',
    photo_url VARCHAR(512) NOT NULL DEFAULT '',
    note VARCHAR(500) NOT NULL DEFAULT '',
    PRIMARY KEY (id),
    KEY idx_pickup_events_operation_student (operation_student_id, event_at),
    KEY idx_pickup_events_student (organization_id, student_id, event_at),
    CONSTRAINT fk_pickup_events_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_pickup_events_operation FOREIGN KEY (operation_id) REFERENCES pickup_operations (id),
    CONSTRAINT fk_pickup_events_operation_student FOREIGN KEY (operation_student_id) REFERENCES pickup_operation_students (id),
    CONSTRAINT fk_pickup_events_student FOREIGN KEY (student_id) REFERENCES students (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE notifications (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    student_id BIGINT UNSIGNED NOT NULL,
    operation_id BIGINT UNSIGNED NULL,
    event_id BIGINT UNSIGNED NULL,
    recipient_type VARCHAR(32) NOT NULL DEFAULT 'parent',
    kind VARCHAR(32) NOT NULL,
    title VARCHAR(128) NOT NULL,
    content VARCHAR(500) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    sent_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_notifications_pending (organization_id, status, created_at),
    KEY idx_notifications_student (organization_id, student_id, created_at),
    CONSTRAINT fk_notifications_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_notifications_student FOREIGN KEY (student_id) REFERENCES students (id),
    CONSTRAINT fk_notifications_operation FOREIGN KEY (operation_id) REFERENCES pickup_operations (id),
    CONSTRAINT fk_notifications_event FOREIGN KEY (event_id) REFERENCES pickup_events (id),
    CONSTRAINT chk_notifications_recipient CHECK (recipient_type IN ('parent')),
    CONSTRAINT chk_notifications_status CHECK (status IN ('pending', 'sent', 'failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE notifications;
DROP TABLE pickup_events;
DROP TABLE pickup_operation_students;
DROP TABLE pickup_operations;
