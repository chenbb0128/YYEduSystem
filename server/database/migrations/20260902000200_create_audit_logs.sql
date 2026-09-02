-- +goose Up
CREATE TABLE audit_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    actor_type VARCHAR(24) NOT NULL,
    actor_id BIGINT UNSIGNED NULL,
    action VARCHAR(64) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id BIGINT UNSIGNED NULL,
    metadata_json JSON NOT NULL,
    request_id VARCHAR(64) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_audit_logs_org_created (organization_id, created_at, id),
    KEY idx_audit_logs_resource (organization_id, resource_type, resource_id, created_at),
    KEY idx_audit_logs_actor (organization_id, actor_type, actor_id, created_at),
    CONSTRAINT fk_audit_logs_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT chk_audit_logs_actor_type CHECK (actor_type IN ('staff', 'parent', 'system', 'anonymous'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE audit_logs;
