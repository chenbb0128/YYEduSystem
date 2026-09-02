-- +goose Up
CREATE TABLE daily_summaries (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    summary_date DATE NOT NULL,
    content VARCHAR(4000) NOT NULL,
    child_updates_json TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    created_by_user_id BIGINT UNSIGNED NULL,
    created_by_name VARCHAR(64) NOT NULL DEFAULT '',
    generated_at DATETIME(3) NULL,
    published_at DATETIME(3) NULL,
    closed_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_daily_summaries_day (organization_id, summary_date),
    KEY idx_daily_summaries_status_date (organization_id, status, summary_date),
    CONSTRAINT fk_daily_summaries_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_daily_summaries_creator FOREIGN KEY (created_by_user_id) REFERENCES users (id),
    CONSTRAINT chk_daily_summaries_status CHECK (status IN ('draft', 'published', 'closed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE daily_summaries;
