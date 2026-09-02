-- +goose Up
CREATE TABLE parent_child_applications (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    parent_account_id BIGINT UNSIGNED NOT NULL,
    student_id BIGINT UNSIGNED NULL,
    school_id BIGINT UNSIGNED NULL,
    school_class_id BIGINT UNSIGNED NULL,
    student_name VARCHAR(64) NOT NULL,
    school_name_input VARCHAR(128) NOT NULL DEFAULT '',
    grade_input VARCHAR(32) NOT NULL DEFAULT '',
    class_name_input VARCHAR(64) NOT NULL DEFAULT '',
    grade VARCHAR(32) NOT NULL DEFAULT '',
    class_name VARCHAR(64) NOT NULL DEFAULT '',
    guardian_name VARCHAR(64) NOT NULL DEFAULT '',
    guardian_phone VARCHAR(32) NOT NULL,
    relationship VARCHAR(32) NOT NULL DEFAULT '家长',
    notes VARCHAR(500) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    review_note VARCHAR(500) NOT NULL DEFAULT '',
    reviewed_by_user_id BIGINT UNSIGNED NULL,
    reviewed_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_parent_child_applications_parent (organization_id, parent_account_id, created_at),
    KEY idx_parent_child_applications_review (organization_id, status, school_class_id, created_at),
    KEY idx_parent_child_applications_student (organization_id, student_id),
    CONSTRAINT fk_parent_child_applications_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_parent_child_applications_parent FOREIGN KEY (parent_account_id) REFERENCES parent_accounts (id),
    CONSTRAINT fk_parent_child_applications_student FOREIGN KEY (student_id) REFERENCES students (id),
    CONSTRAINT fk_parent_child_applications_school FOREIGN KEY (school_id) REFERENCES schools (id),
    CONSTRAINT fk_parent_child_applications_school_class FOREIGN KEY (school_class_id) REFERENCES school_classes (id),
    CONSTRAINT fk_parent_child_applications_reviewer FOREIGN KEY (reviewed_by_user_id) REFERENCES users (id),
    CONSTRAINT chk_parent_child_applications_status CHECK (status IN ('pending', 'needs_info', 'approved', 'rejected'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE parent_child_applications;
