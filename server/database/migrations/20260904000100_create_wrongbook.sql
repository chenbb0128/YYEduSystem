-- +goose Up
CREATE TABLE wrong_questions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    student_id BIGINT UNSIGNED NOT NULL,
    subject VARCHAR(64) NOT NULL DEFAULT '',
    question_text VARCHAR(2000) NOT NULL,
    answer_text VARCHAR(1000) NOT NULL DEFAULT '',
    explanation VARCHAR(2000) NOT NULL DEFAULT '',
    knowledge_point VARCHAR(255) NOT NULL DEFAULT '',
    source_image_url VARCHAR(512) NOT NULL DEFAULT '',
    source_homework_task_id BIGINT UNSIGNED NULL,
    teacher_note VARCHAR(500) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_by_user_id BIGINT UNSIGNED NULL,
    created_by_name VARCHAR(64) NOT NULL DEFAULT '',
    last_reviewed_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_wrong_questions_student (organization_id, student_id, status, created_at),
    KEY idx_wrong_questions_subject (organization_id, subject, status),
    KEY idx_wrong_questions_homework (organization_id, source_homework_task_id),
    CONSTRAINT fk_wrong_questions_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_wrong_questions_student FOREIGN KEY (student_id) REFERENCES students (id),
    CONSTRAINT fk_wrong_questions_homework_task FOREIGN KEY (source_homework_task_id) REFERENCES homework_tasks (id),
    CONSTRAINT fk_wrong_questions_creator FOREIGN KEY (created_by_user_id) REFERENCES users (id),
    CONSTRAINT chk_wrong_questions_status CHECK (status IN ('active', 'mastered', 'archived'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wrong_papers (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    student_id BIGINT UNSIGNED NOT NULL,
    title VARCHAR(128) NOT NULL,
    source VARCHAR(32) NOT NULL DEFAULT 'teacher',
    status VARCHAR(32) NOT NULL DEFAULT 'generated',
    generated_by_type VARCHAR(32) NOT NULL DEFAULT 'staff',
    generated_by_user_id BIGINT UNSIGNED NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_wrong_papers_student (organization_id, student_id, created_at),
    CONSTRAINT fk_wrong_papers_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_wrong_papers_student FOREIGN KEY (student_id) REFERENCES students (id),
    CONSTRAINT chk_wrong_papers_status CHECK (status IN ('generated', 'assigned', 'archived')),
    CONSTRAINT chk_wrong_papers_source CHECK (source IN ('teacher', 'parent', 'system')),
    CONSTRAINT chk_wrong_papers_generated_by_type CHECK (generated_by_type IN ('staff', 'parent', 'system'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wrong_paper_questions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    paper_id BIGINT UNSIGNED NOT NULL,
    question_id BIGINT UNSIGNED NOT NULL,
    sort_order INT UNSIGNED NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_wrong_paper_questions_item (paper_id, question_id),
    KEY idx_wrong_paper_questions_paper (organization_id, paper_id, sort_order),
    CONSTRAINT fk_wrong_paper_questions_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_wrong_paper_questions_paper FOREIGN KEY (paper_id) REFERENCES wrong_papers (id),
    CONSTRAINT fk_wrong_paper_questions_question FOREIGN KEY (question_id) REFERENCES wrong_questions (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE wrong_paper_questions;
DROP TABLE wrong_papers;
DROP TABLE wrong_questions;
