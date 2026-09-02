-- +goose Up
CREATE TABLE parent_message_subscriptions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    parent_account_id BIGINT UNSIGNED NOT NULL,
    message_kind VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_parent_message_subscriptions_kind (organization_id, parent_account_id, message_kind),
    KEY idx_parent_message_subscriptions_parent (organization_id, parent_account_id),
    CONSTRAINT fk_parent_message_subscriptions_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_parent_message_subscriptions_parent FOREIGN KEY (parent_account_id) REFERENCES parent_accounts (id),
    CONSTRAINT chk_parent_message_subscriptions_status CHECK (status IN ('accept', 'reject', 'ban', 'filter', 'unknown'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE notifications
    ADD COLUMN delivery_attempts INT UNSIGNED NOT NULL DEFAULT 0 AFTER sent_at,
    ADD COLUMN last_attempt_at DATETIME(3) NULL AFTER delivery_attempts,
    ADD COLUMN delivery_error VARCHAR(500) NOT NULL DEFAULT '' AFTER last_attempt_at,
    ADD COLUMN next_retry_at DATETIME(3) NULL AFTER delivery_error;

-- +goose Down
ALTER TABLE notifications
    DROP COLUMN next_retry_at,
    DROP COLUMN delivery_error,
    DROP COLUMN last_attempt_at,
    DROP COLUMN delivery_attempts;
DROP TABLE parent_message_subscriptions;
