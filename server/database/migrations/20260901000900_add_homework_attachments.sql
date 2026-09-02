-- +goose Up
ALTER TABLE homework_tasks
    ADD COLUMN attachment_urls JSON NULL AFTER content;

UPDATE homework_tasks
SET attachment_urls = JSON_ARRAY()
WHERE attachment_urls IS NULL;

ALTER TABLE homework_tasks
    MODIFY COLUMN attachment_urls JSON NOT NULL;

-- +goose Down
ALTER TABLE homework_tasks
    DROP COLUMN attachment_urls;
