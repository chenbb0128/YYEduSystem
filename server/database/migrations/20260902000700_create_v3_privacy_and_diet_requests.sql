-- +goose Up
CREATE TABLE diet_note_change_requests (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    student_id BIGINT UNSIGNED NOT NULL,
    parent_account_id BIGINT UNSIGNED NOT NULL,
    current_note VARCHAR(500) NOT NULL DEFAULT '',
    requested_note VARCHAR(500) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    review_note VARCHAR(500) NOT NULL DEFAULT '',
    reviewed_by_user_id BIGINT UNSIGNED NULL,
    reviewed_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_diet_note_change_requests_status (organization_id, status, created_at),
    KEY idx_diet_note_change_requests_student (organization_id, student_id, created_at),
    KEY idx_diet_note_change_requests_parent (organization_id, parent_account_id, created_at),
    CONSTRAINT fk_diet_note_change_requests_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_diet_note_change_requests_student FOREIGN KEY (student_id) REFERENCES students (id),
    CONSTRAINT fk_diet_note_change_requests_parent FOREIGN KEY (parent_account_id) REFERENCES parent_accounts (id),
    CONSTRAINT fk_diet_note_change_requests_reviewer FOREIGN KEY (reviewed_by_user_id) REFERENCES users (id),
    CONSTRAINT chk_diet_note_change_requests_status CHECK (status IN ('pending', 'approved', 'rejected'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE parent_privacy_consents (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    parent_account_id BIGINT UNSIGNED NOT NULL,
    policy_version VARCHAR(128) NOT NULL,
    consented_at DATETIME(3) NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_parent_privacy_consents_version (organization_id, parent_account_id, policy_version),
    KEY idx_parent_privacy_consents_latest (organization_id, parent_account_id, consented_at),
    CONSTRAINT fk_parent_privacy_consents_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_parent_privacy_consents_parent FOREIGN KEY (parent_account_id) REFERENCES parent_accounts (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE parent_privacy_consents;
DROP TABLE diet_note_change_requests;
