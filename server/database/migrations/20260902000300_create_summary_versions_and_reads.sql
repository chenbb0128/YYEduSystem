-- +goose Up
ALTER TABLE daily_summaries
    ADD COLUMN current_version INT UNSIGNED NOT NULL DEFAULT 1 AFTER status,
    ADD COLUMN withdrawn_at DATETIME(3) NULL AFTER closed_at,
    ADD COLUMN withdrawal_reason VARCHAR(500) NOT NULL DEFAULT '' AFTER withdrawn_at,
    ADD COLUMN correction_reason VARCHAR(500) NOT NULL DEFAULT '' AFTER withdrawal_reason;

ALTER TABLE daily_summaries
    DROP CHECK chk_daily_summaries_status,
    ADD CONSTRAINT chk_daily_summaries_status CHECK (status IN ('draft', 'published', 'closed', 'withdrawn'));

CREATE TABLE daily_summary_versions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    daily_summary_id BIGINT UNSIGNED NOT NULL,
    version INT UNSIGNED NOT NULL,
    action VARCHAR(32) NOT NULL,
    content VARCHAR(4000) NOT NULL,
    child_updates_json TEXT NOT NULL,
    reason VARCHAR(500) NOT NULL DEFAULT '',
    created_by_user_id BIGINT UNSIGNED NULL,
    created_by_name VARCHAR(64) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_daily_summary_versions_snapshot (daily_summary_id, version, action),
    KEY idx_daily_summary_versions_summary (organization_id, daily_summary_id, version),
    CONSTRAINT fk_daily_summary_versions_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_daily_summary_versions_summary FOREIGN KEY (daily_summary_id) REFERENCES daily_summaries (id),
    CONSTRAINT fk_daily_summary_versions_creator FOREIGN KEY (created_by_user_id) REFERENCES users (id),
    CONSTRAINT chk_daily_summary_versions_action CHECK (action IN ('generated', 'updated', 'published', 'closed', 'withdrawn', 'corrected'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO daily_summary_versions (
    organization_id, daily_summary_id, version, action, content, child_updates_json,
    reason, created_by_user_id, created_by_name, created_at
)
SELECT organization_id, id, 1, 'generated', content, child_updates_json,
       '', created_by_user_id, created_by_name, created_at
FROM daily_summaries;

CREATE TABLE daily_summary_reads (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    daily_summary_id BIGINT UNSIGNED NOT NULL,
    parent_account_id BIGINT UNSIGNED NOT NULL,
    read_version INT UNSIGNED NOT NULL DEFAULT 1,
    read_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_daily_summary_reads_parent (daily_summary_id, parent_account_id),
    KEY idx_daily_summary_reads_parent (organization_id, parent_account_id, read_at),
    CONSTRAINT fk_daily_summary_reads_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_daily_summary_reads_summary FOREIGN KEY (daily_summary_id) REFERENCES daily_summaries (id),
    CONSTRAINT fk_daily_summary_reads_parent FOREIGN KEY (parent_account_id) REFERENCES parent_accounts (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE daily_summary_reads;
DROP TABLE daily_summary_versions;
ALTER TABLE daily_summaries
    DROP CHECK chk_daily_summaries_status,
    ADD CONSTRAINT chk_daily_summaries_status CHECK (status IN ('draft', 'published', 'closed')),
    DROP COLUMN correction_reason,
    DROP COLUMN withdrawal_reason,
    DROP COLUMN withdrawn_at,
    DROP COLUMN current_version;
