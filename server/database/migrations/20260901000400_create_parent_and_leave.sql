-- +goose Up
CREATE TABLE parent_accounts (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    openid VARCHAR(128) NOT NULL,
    nickname VARCHAR(64) NOT NULL DEFAULT '',
    avatar VARCHAR(512) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_parent_accounts_openid (organization_id, openid),
    KEY idx_parent_accounts_status (organization_id, status),
    CONSTRAINT fk_parent_accounts_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT chk_parent_accounts_status CHECK (status IN ('active', 'disabled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE parent_student_bindings (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    parent_account_id BIGINT UNSIGNED NOT NULL,
    student_id BIGINT UNSIGNED NOT NULL,
    relationship VARCHAR(32) NOT NULL DEFAULT 'guardian',
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_parent_student_binding (organization_id, parent_account_id, student_id),
    KEY idx_parent_student_bindings_student (organization_id, student_id, status),
    CONSTRAINT fk_parent_student_bindings_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_parent_student_bindings_parent FOREIGN KEY (parent_account_id) REFERENCES parent_accounts (id),
    CONSTRAINT fk_parent_student_bindings_student FOREIGN KEY (student_id) REFERENCES students (id),
    CONSTRAINT chk_parent_student_bindings_status CHECK (status IN ('active', 'inactive'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE leave_requests (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    student_id BIGINT UNSIGNED NOT NULL,
    parent_account_id BIGINT UNSIGNED NULL,
    submitted_by_type VARCHAR(32) NOT NULL,
    submitted_by_user_id BIGINT UNSIGNED NULL,
    leave_date DATE NOT NULL,
    reason VARCHAR(500) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    teacher_note VARCHAR(500) NOT NULL DEFAULT '',
    reviewed_by_user_id BIGINT UNSIGNED NULL,
    reviewed_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_leave_requests_date_status (organization_id, leave_date, status),
    KEY idx_leave_requests_parent_date (organization_id, parent_account_id, leave_date),
    KEY idx_leave_requests_student_date (organization_id, student_id, leave_date),
    CONSTRAINT fk_leave_requests_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_leave_requests_student FOREIGN KEY (student_id) REFERENCES students (id),
    CONSTRAINT fk_leave_requests_parent FOREIGN KEY (parent_account_id) REFERENCES parent_accounts (id),
    CONSTRAINT fk_leave_requests_submitter FOREIGN KEY (submitted_by_user_id) REFERENCES users (id),
    CONSTRAINT fk_leave_requests_reviewer FOREIGN KEY (reviewed_by_user_id) REFERENCES users (id),
    CONSTRAINT chk_leave_requests_submitter CHECK (submitted_by_type IN ('parent', 'teacher')),
    CONSTRAINT chk_leave_requests_status CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE leave_requests;
DROP TABLE parent_student_bindings;
DROP TABLE parent_accounts;
