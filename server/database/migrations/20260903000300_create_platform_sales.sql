-- +goose Up
CREATE TABLE organization_invites (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code_hash CHAR(64) NOT NULL,
    code_hint VARCHAR(16) NOT NULL,
    max_uses INT UNSIGNED NOT NULL DEFAULT 1,
    used_count INT UNSIGNED NOT NULL DEFAULT 0,
    expires_at DATETIME(3) NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    note VARCHAR(255) NOT NULL DEFAULT '',
    created_by_user_id BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_organization_invites_code_hash (code_hash),
    KEY idx_organization_invites_status (status, expires_at),
    CONSTRAINT fk_organization_invites_creator FOREIGN KEY (created_by_user_id) REFERENCES users (id),
    CONSTRAINT chk_organization_invites_status CHECK (status IN ('active', 'revoked', 'exhausted')),
    CONSTRAINT chk_organization_invites_uses CHECK (max_uses > 0 AND used_count <= max_uses)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE organization_registrations (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    invite_id BIGINT UNSIGNED NOT NULL,
    organization_id BIGINT UNSIGNED NULL,
    organization_name VARCHAR(128) NOT NULL,
    slug VARCHAR(64) NOT NULL,
    contact_name VARCHAR(64) NOT NULL DEFAULT '',
    contact_phone VARCHAR(32) NOT NULL DEFAULT '',
    admin_username VARCHAR(64) NOT NULL,
    admin_password_hash VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    review_note VARCHAR(500) NOT NULL DEFAULT '',
    reviewed_by_user_id BIGINT UNSIGNED NULL,
    reviewed_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_organization_registrations_status (status, created_at),
    KEY idx_organization_registrations_invite (invite_id),
    CONSTRAINT fk_organization_registrations_invite FOREIGN KEY (invite_id) REFERENCES organization_invites (id),
    CONSTRAINT fk_organization_registrations_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_organization_registrations_reviewer FOREIGN KEY (reviewed_by_user_id) REFERENCES users (id),
    CONSTRAINT chk_organization_registrations_status CHECK (status IN ('pending', 'approved', 'rejected'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE organization_registrations;
DROP TABLE organization_invites;
