-- 0010_skills.sql
-- Persisted catalogue of skills the daemon knows about. Skills live as
-- markdown files (with YAML frontmatter) in:
--   ~/.claude/skills/<name>/SKILL.md           ← global
--   <project>/.claude/skills/<name>/SKILL.md   ← project-scoped
--   data/skill-registry/<source>/<name>/...    ← synced from a remote registry
--
-- The DB row is a denormalised projection of those files: parsed frontmatter
-- + body, kept fresh by the sync routine. Source 'local' is what's
-- discovered under ~/.claude/skills/ on every sync; 'project:<name>' is the
-- in-repo skills folder; named sources come from the registries config.

CREATE TABLE IF NOT EXISTS skills (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  name         TEXT NOT NULL,                 -- slug, e.g. 'topic-routing'
  source       TEXT NOT NULL,                 -- 'local' | 'project:<slug>' | 'registry:<name>'
  description  TEXT,
  role_hint    TEXT,                          -- optional: for which role is this skill (executor, verifier, …)
  version      TEXT,                          -- semver from frontmatter (default '0.0.0')
  frontmatter  TEXT,                          -- raw YAML block as JSON-encoded map
  body         TEXT NOT NULL,                 -- markdown content
  path         TEXT NOT NULL,                 -- absolute path on disk
  pulled_at    INTEGER NOT NULL,              -- when sync last touched it
  updated_at   INTEGER NOT NULL,
  UNIQUE(source, name)
);

CREATE INDEX IF NOT EXISTS idx_skills_name ON skills(name);
CREATE INDEX IF NOT EXISTS idx_skills_source ON skills(source);
