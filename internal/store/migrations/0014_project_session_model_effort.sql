-- Project sessions are engine-scoped but model/effort may vary per session.
-- Existing rows get NULLs and the server falls back to engine defaults.
ALTER TABLE project_sessions ADD COLUMN model TEXT;
ALTER TABLE project_sessions ADD COLUMN reasoning_effort TEXT;
