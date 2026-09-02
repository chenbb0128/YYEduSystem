-- +goose Up
CREATE TABLE media_assets (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    object_key VARCHAR(512) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id BIGINT UNSIGNED NULL,
    owner_type VARCHAR(32) NOT NULL DEFAULT 'organization',
    owner_id BIGINT UNSIGNED NULL,
    content_type VARCHAR(128) NOT NULL,
    size_bytes BIGINT UNSIGNED NOT NULL,
    sha256_hex CHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    retention_until DATETIME(3) NULL,
    created_by_user_id BIGINT UNSIGNED NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    deleted_at DATETIME(3) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_media_assets_object_key (organization_id, object_key),
    KEY idx_media_assets_resource (organization_id, resource_type, resource_id, created_at),
    KEY idx_media_assets_retention (organization_id, status, retention_until),
    CONSTRAINT fk_media_assets_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_media_assets_creator FOREIGN KEY (created_by_user_id) REFERENCES users (id),
    CONSTRAINT chk_media_assets_status CHECK (status IN ('active', 'deleted')),
    CONSTRAINT chk_media_assets_owner_type CHECK (owner_type IN ('organization', 'staff', 'parent', 'system'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE media_assets;
