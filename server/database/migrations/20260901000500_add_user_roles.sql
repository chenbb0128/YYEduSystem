-- +goose Up
ALTER TABLE users
    ADD COLUMN role VARCHAR(32) NOT NULL DEFAULT 'teacher' AFTER password_hash,
    ADD CONSTRAINT chk_users_role CHECK (role IN ('admin', 'teacher', 'editor', 'viewer'));

UPDATE users SET role = 'admin' WHERE username = 'admin';

-- +goose Down
ALTER TABLE users
    DROP CONSTRAINT chk_users_role,
    DROP COLUMN role;
