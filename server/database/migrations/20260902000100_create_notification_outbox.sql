-- +goose Up
CREATE TABLE outbox_events (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id BIGINT UNSIGNED NOT NULL,
    notification_id BIGINT UNSIGNED NOT NULL,
    payload_json JSON NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'pending',
    attempts INT UNSIGNED NOT NULL DEFAULT 0,
    available_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    locked_at DATETIME(3) NULL,
    processed_at DATETIME(3) NULL,
    last_error VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_outbox_notification (notification_id),
    KEY idx_outbox_ready (status, available_at, locked_at, id),
    KEY idx_outbox_organization (organization_id, created_at),
    CONSTRAINT fk_outbox_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_outbox_notification FOREIGN KEY (notification_id) REFERENCES notifications (id),
    CONSTRAINT chk_outbox_status CHECK (status IN ('pending', 'processing', 'processed', 'failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE notification_delivery_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    notification_id BIGINT UNSIGNED NOT NULL,
    parent_account_id BIGINT UNSIGNED NOT NULL,
    message_kind VARCHAR(32) NOT NULL,
    template_id VARCHAR(128) NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'pending',
    attempts INT UNSIGNED NOT NULL DEFAULT 0,
    last_attempt_at DATETIME(3) NULL,
    sent_at DATETIME(3) NULL,
    next_retry_at DATETIME(3) NULL,
    delivery_error VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_notification_delivery_recipient (notification_id, parent_account_id, message_kind, template_id),
    KEY idx_notification_delivery_status (organization_id, status, next_retry_at, id),
    KEY idx_notification_delivery_notification (organization_id, notification_id, id),
    CONSTRAINT fk_notification_delivery_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_notification_delivery_notification FOREIGN KEY (notification_id) REFERENCES notifications (id),
    CONSTRAINT fk_notification_delivery_parent FOREIGN KEY (parent_account_id) REFERENCES parent_accounts (id),
    CONSTRAINT chk_notification_delivery_status CHECK (status IN ('pending', 'sent', 'failed', 'skipped'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE notification_delivery_logs;
DROP TABLE outbox_events;
