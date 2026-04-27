-- 0011_skill_improvements.sql
-- Self-improving skills: any agent can drop a proposed improvement after a
-- run; the user reviews + approves/rejects from the UI. Approved ones bump
-- the skill version and (optionally) push to the upstream repo.

CREATE TABLE IF NOT EXISTS skill_improvements (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  skill_name      TEXT NOT NULL,
  source          TEXT NOT NULL,             -- 'local' | 'project:<slug>' | 'registry:<name>'
  proposed_by     TEXT,                      -- agent name / 'user' / 'main-agent'
  rationale       TEXT NOT NULL,             -- why this improvement matters (1-2 paragraphs)
  patch           TEXT NOT NULL,             -- markdown to append OR a unified diff against the SKILL.md body
  patch_kind      TEXT NOT NULL DEFAULT 'append' CHECK(patch_kind IN ('append','replace','diff')),
  status          TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','approved','rejected')),
  resolved_by     TEXT,                      -- 'user' / agent name when approved/rejected
  resolved_at     INTEGER,
  created_at      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_skill_improvements_pending
  ON skill_improvements(status, skill_name) WHERE status = 'pending';
