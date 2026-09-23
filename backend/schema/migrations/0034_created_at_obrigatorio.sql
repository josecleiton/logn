UPDATE users SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL;
UPDATE skill_nodes SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL;
UPDATE refresh_tokens SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL;

ALTER TABLE users ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE skill_nodes ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE refresh_tokens ALTER COLUMN created_at SET NOT NULL;
