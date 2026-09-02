-- +goose Up
CREATE TABLE meal_plans (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    meal_date DATE NOT NULL,
    menu_text VARCHAR(2000) NOT NULL,
    photo_url VARCHAR(512) NOT NULL DEFAULT '',
    adjustment_note VARCHAR(500) NOT NULL DEFAULT '',
    created_by_user_id BIGINT UNSIGNED NULL,
    created_by_name VARCHAR(64) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_meal_plans_day (organization_id, meal_date),
    KEY idx_meal_plans_status_date (organization_id, status, meal_date),
    CONSTRAINT fk_meal_plans_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_meal_plans_creator FOREIGN KEY (created_by_user_id) REFERENCES users (id),
    CONSTRAINT chk_meal_plans_status CHECK (status IN ('active', 'closed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE student_diet_notes (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    student_id BIGINT UNSIGNED NOT NULL,
    note VARCHAR(500) NOT NULL DEFAULT '',
    updated_by_user_id BIGINT UNSIGNED NULL,
    updated_by_name VARCHAR(64) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_student_diet_notes_student (organization_id, student_id),
    CONSTRAINT fk_student_diet_notes_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_student_diet_notes_student FOREIGN KEY (student_id) REFERENCES students (id),
    CONSTRAINT fk_student_diet_notes_updater FOREIGN KEY (updated_by_user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE student_diet_notes;
DROP TABLE meal_plans;
