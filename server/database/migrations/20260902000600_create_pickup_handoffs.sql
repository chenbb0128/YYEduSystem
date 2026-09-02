-- +goose Up
CREATE TABLE pickup_operation_handoffs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    operation_id BIGINT UNSIGNED NOT NULL,
    from_teacher_user_id BIGINT UNSIGNED NULL,
    from_teacher_name VARCHAR(64) NOT NULL DEFAULT '',
    to_teacher_user_id BIGINT UNSIGNED NOT NULL,
    to_teacher_name VARCHAR(64) NOT NULL DEFAULT '',
    teacher_role VARCHAR(32) NOT NULL DEFAULT 'collaborator',
    note VARCHAR(500) NOT NULL DEFAULT '',
    handoff_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    created_by_user_id BIGINT UNSIGNED NULL,
    created_by_name VARCHAR(64) NOT NULL DEFAULT '',
    PRIMARY KEY (id),
    KEY idx_pickup_handoffs_operation (organization_id, operation_id, handoff_at, id),
    KEY idx_pickup_handoffs_to_teacher (organization_id, to_teacher_user_id, handoff_at),
    CONSTRAINT fk_pickup_handoffs_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_pickup_handoffs_operation FOREIGN KEY (operation_id) REFERENCES pickup_operations (id),
    CONSTRAINT fk_pickup_handoffs_from_teacher FOREIGN KEY (from_teacher_user_id) REFERENCES users (id),
    CONSTRAINT fk_pickup_handoffs_to_teacher FOREIGN KEY (to_teacher_user_id) REFERENCES users (id),
    CONSTRAINT fk_pickup_handoffs_creator FOREIGN KEY (created_by_user_id) REFERENCES users (id),
    CONSTRAINT chk_pickup_handoffs_role CHECK (teacher_role IN ('lead', 'collaborator', 'substitute'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE pickup_operation_handoffs;
