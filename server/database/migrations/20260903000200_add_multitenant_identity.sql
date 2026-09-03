-- +goose Up
ALTER TABLE organizations
    ADD COLUMN contact_name VARCHAR(64) NOT NULL DEFAULT '' AFTER slug,
    ADD COLUMN contact_phone VARCHAR(32) NOT NULL DEFAULT '' AFTER contact_name,
    ADD COLUMN authorized_until DATE NULL AFTER contact_phone,
    DROP CHECK chk_organizations_status,
    ADD CONSTRAINT chk_organizations_status CHECK (status IN ('pending', 'active', 'disabled'));

ALTER TABLE users
    ADD COLUMN organization_id BIGINT UNSIGNED NOT NULL DEFAULT 1 AFTER id,
    DROP CHECK chk_users_role,
    ADD CONSTRAINT chk_users_role CHECK (role IN ('platform_admin', 'admin', 'teacher', 'editor', 'viewer')),
    ADD KEY idx_users_organization_status (organization_id, status),
    ADD CONSTRAINT fk_users_organization FOREIGN KEY (organization_id) REFERENCES organizations (id);

-- +goose Down
ALTER TABLE users
    DROP FOREIGN KEY fk_users_organization,
    DROP KEY idx_users_organization_status,
    DROP CHECK chk_users_role,
    ADD CONSTRAINT chk_users_role CHECK (role IN ('admin', 'teacher', 'editor', 'viewer')),
    DROP COLUMN organization_id;

ALTER TABLE organizations
    DROP CHECK chk_organizations_status,
    ADD CONSTRAINT chk_organizations_status CHECK (status IN ('active', 'disabled')),
    DROP COLUMN authorized_until,
    DROP COLUMN contact_phone,
    DROP COLUMN contact_name;
