-- +goose Up
CREATE TABLE homework_tasks (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    homework_date DATE NOT NULL,
    school_id BIGINT UNSIGNED NOT NULL,
    school_class_id BIGINT UNSIGNED NOT NULL,
    subject VARCHAR(64) NOT NULL DEFAULT '',
    content VARCHAR(1000) NOT NULL,
    created_by_user_id BIGINT UNSIGNED NULL,
    creator_name VARCHAR(64) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_homework_tasks_date_class (organization_id, homework_date, school_class_id),
    KEY idx_homework_tasks_status (organization_id, status, homework_date),
    CONSTRAINT fk_homework_tasks_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_homework_tasks_school FOREIGN KEY (school_id) REFERENCES schools (id),
    CONSTRAINT fk_homework_tasks_school_class FOREIGN KEY (school_class_id) REFERENCES school_classes (id),
    CONSTRAINT fk_homework_tasks_creator FOREIGN KEY (created_by_user_id) REFERENCES users (id),
    CONSTRAINT chk_homework_tasks_status CHECK (status IN ('active', 'cancelled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE homework_task_students (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    task_id BIGINT UNSIGNED NOT NULL,
    student_id BIGINT UNSIGNED NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    correction_note VARCHAR(500) NOT NULL DEFAULT '',
    reviewed_by_user_id BIGINT UNSIGNED NULL,
    reviewed_at DATETIME(3) NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_homework_task_students_task_student (task_id, student_id),
    KEY idx_homework_task_students_student (organization_id, student_id, status),
    CONSTRAINT fk_homework_task_students_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_homework_task_students_task FOREIGN KEY (task_id) REFERENCES homework_tasks (id),
    CONSTRAINT fk_homework_task_students_student FOREIGN KEY (student_id) REFERENCES students (id),
    CONSTRAINT fk_homework_task_students_reviewer FOREIGN KEY (reviewed_by_user_id) REFERENCES users (id),
    CONSTRAINT chk_homework_task_students_status CHECK (status IN ('pending', 'completed', 'incomplete', 'not_submitted'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE homework_task_students;
DROP TABLE homework_tasks;
