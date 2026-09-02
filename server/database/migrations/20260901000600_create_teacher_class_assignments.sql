-- +goose Up
CREATE TABLE teacher_class_assignments (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    teacher_user_id BIGINT UNSIGNED NOT NULL,
    school_class_id BIGINT UNSIGNED NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_teacher_class_assignments_teacher_class (organization_id, teacher_user_id, school_class_id),
    KEY idx_teacher_class_assignments_teacher (organization_id, teacher_user_id, status),
    KEY idx_teacher_class_assignments_class (organization_id, school_class_id, status),
    CONSTRAINT fk_teacher_class_assignments_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_teacher_class_assignments_teacher FOREIGN KEY (teacher_user_id) REFERENCES users (id),
    CONSTRAINT fk_teacher_class_assignments_school_class FOREIGN KEY (school_class_id) REFERENCES school_classes (id),
    CONSTRAINT chk_teacher_class_assignments_status CHECK (status IN ('active', 'disabled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE teacher_class_assignments;
