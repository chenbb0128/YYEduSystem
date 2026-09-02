-- +goose Up
ALTER TABLE parent_message_subscriptions
    ADD COLUMN authorized_at DATETIME(3) NULL AFTER status,
    ADD COLUMN template_version VARCHAR(128) NOT NULL DEFAULT '' AFTER authorized_at;

UPDATE parent_message_subscriptions
SET authorized_at = updated_at
WHERE status = 'accept' AND authorized_at IS NULL;

-- +goose Down
ALTER TABLE parent_message_subscriptions
    DROP COLUMN template_version,
    DROP COLUMN authorized_at;
