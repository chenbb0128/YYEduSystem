-- +goose Up
CREATE TABLE organizations (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(128) NOT NULL,
    slug VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_organizations_slug (slug),
    CONSTRAINT chk_organizations_status CHECK (status IN ('active', 'disabled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO organizations (name, slug) VALUES ('我的托管班', 'default');

CREATE TABLE schools (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(128) NOT NULL,
    address VARCHAR(255) NOT NULL DEFAULT '',
    contact_phone VARCHAR(32) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_schools_organization_name (organization_id, name),
    KEY idx_schools_organization_status (organization_id, status),
    CONSTRAINT fk_schools_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT chk_schools_status CHECK (status IN ('active', 'disabled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE academic_terms (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(128) NOT NULL,
    starts_on DATE NOT NULL,
    ends_on DATE NOT NULL,
    is_current BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_terms_organization_name (organization_id, name),
    KEY idx_terms_organization_current (organization_id, is_current),
    CONSTRAINT fk_terms_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT chk_terms_status CHECK (status IN ('active', 'disabled')),
    CONSTRAINT chk_terms_date_range CHECK (starts_on <= ends_on)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE school_classes (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    school_id BIGINT UNSIGNED NOT NULL,
    term_id BIGINT UNSIGNED NOT NULL,
    grade VARCHAR(32) NOT NULL,
    name VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_school_classes_identity (organization_id, school_id, term_id, grade, name),
    KEY idx_school_classes_organization_term (organization_id, term_id),
    CONSTRAINT fk_school_classes_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_school_classes_school FOREIGN KEY (school_id) REFERENCES schools (id),
    CONSTRAINT fk_school_classes_term FOREIGN KEY (term_id) REFERENCES academic_terms (id),
    CONSTRAINT chk_school_classes_status CHECK (status IN ('active', 'disabled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE care_classes (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(128) NOT NULL,
    capacity INT UNSIGNED NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_care_classes_organization_name (organization_id, name),
    KEY idx_care_classes_organization_status (organization_id, status),
    CONSTRAINT fk_care_classes_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT chk_care_classes_status CHECK (status IN ('active', 'disabled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE students (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    school_id BIGINT UNSIGNED NOT NULL,
    term_id BIGINT UNSIGNED NOT NULL,
    school_class_id BIGINT UNSIGNED NOT NULL,
    care_class_id BIGINT UNSIGNED NULL,
    name VARCHAR(64) NOT NULL,
    gender VARCHAR(16) NOT NULL DEFAULT 'unknown',
    birth_date DATE NULL,
    student_no VARCHAR(64) NOT NULL DEFAULT '',
    guardian_phone VARCHAR(32) NOT NULL DEFAULT '',
    emergency_contact VARCHAR(64) NOT NULL DEFAULT '',
    emergency_phone VARCHAR(32) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    notes VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    KEY idx_students_organization_status (organization_id, status),
    KEY idx_students_school_class (organization_id, school_class_id),
    KEY idx_students_care_class (organization_id, care_class_id),
    CONSTRAINT fk_students_organization FOREIGN KEY (organization_id) REFERENCES organizations (id),
    CONSTRAINT fk_students_school FOREIGN KEY (school_id) REFERENCES schools (id),
    CONSTRAINT fk_students_term FOREIGN KEY (term_id) REFERENCES academic_terms (id),
    CONSTRAINT fk_students_school_class FOREIGN KEY (school_class_id) REFERENCES school_classes (id),
    CONSTRAINT fk_students_care_class FOREIGN KEY (care_class_id) REFERENCES care_classes (id),
    CONSTRAINT chk_students_gender CHECK (gender IN ('unknown', 'male', 'female')),
    CONSTRAINT chk_students_status CHECK (status IN ('active', 'inactive'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE students;
DROP TABLE care_classes;
DROP TABLE school_classes;
DROP TABLE academic_terms;
DROP TABLE schools;
DROP TABLE organizations;
