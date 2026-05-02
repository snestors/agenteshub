-- Drop the mini-agent subsystem (f-002).
--
-- 0001 introduced agents + agent_schedules + agent_runs to support the
-- "scheduled mini-agent" concept (cron-fired Ollama/Claude jobs that the user
-- could create from /api/agents). The user decided that concept goes away
-- entirely — only the main-agent and project-agent tiers remain. agent_runs
-- ran on every scheduler tick; without the scheduler the table is dead
-- weight.
--
-- Tables that LOOK related but are NOT mini-agent state and stay:
--   - agent_sessions  → engine-scoped session_id store for main-agent and
--                       project-agent resumes (used by SessionsRepo).
--   - agent_records   → schema-on-read JSON facts the main-agent persists
--                       (calorías, ejercicio, etc.). source='agent' is a
--                       legacy column name — the table is still in use.
--   - subagent_runs   → captures of Task-tool sub-spawns from JSONL post-
--                       hoc; produced by the main-agent runtime, not by the
--                       deleted scheduler.

PRAGMA foreign_keys = OFF;

BEGIN;

DROP INDEX IF EXISTS idx_agents_enabled;
DROP INDEX IF EXISTS idx_agents_name;
DROP TABLE IF EXISTS agents;

DROP INDEX IF EXISTS idx_agent_schedules_next_run;
DROP INDEX IF EXISTS idx_agent_schedules_agent;
DROP TABLE IF EXISTS agent_schedules;

DROP INDEX IF EXISTS idx_agent_runs_status;
DROP INDEX IF EXISTS idx_agent_runs_agent;
DROP INDEX IF EXISTS idx_agent_runs_started;
DROP TABLE IF EXISTS agent_runs;

COMMIT;

PRAGMA foreign_keys = ON;
