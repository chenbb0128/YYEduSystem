-- +goose Up
CREATE TABLE pickup_schedules (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    school_id BIGINT UNSIGNED NOT NULL,
    school_class_id BIGINT UNSIGNED NOT NULL,
    care_class_id BIGINT UNSIGNED NULL,
    weekday TINYINT UNSIGNED NOT NULL,
    pickup_mode VARCHAR(32) NOT NULL DEFAULT 'school_pickup',
    teacher_user_id BIGINT UNSIGNED NULL,
    teacher_name VARCHAR(64) NOT NULL DEFAULT '',
    expected_pickup_time VARCHAR(16) NOT NULL DEFAULT '',
    effective_from DATE NOT NULL,
    effective_to DATE NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    notes VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_pickup_schedules_date (organization_id, weekday, effective_from, effective_to, enabled),
    KEY idx_pickup_schedules_class (organization_id, school_class_id, enabled),
    KEY idx_pickup_schedules_teacher (organization_id, teacher_user_id, enabled),
    CONSTRAINT fk_pickup_schedules_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_pickup_schedules_school FOREIGN KEY (school_id) REFERENCES schools (id),
    CONSTRAINT fk_pickup_schedules_school_class FOREIGN KEY (school_class_id) REFERENCES school_classes (id),
    CONSTRAINT fk_pickup_schedules_care_class FOREIGN KEY (care_class_id) REFERENCES care_classes (id),
    CONSTRAINT fk_pickup_schedules_teacher FOREIGN KEY (teacher_user_id) REFERENCES users (id),
    CONSTRAINT chk_pickup_schedules_weekday CHECK (weekday BETWEEN 1 AND 7),
    CONSTRAINT chk_pickup_schedules_mode CHECK (pickup_mode IN ('school_pickup', 'self_arrival')),
    CONSTRAINT chk_pickup_schedules_dates CHECK (effective_to IS NULL OR effective_to >= effective_from)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE pickup_schedules;
