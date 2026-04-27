# Tasks

- [ ] Locate all code paths that run CodexEngine and record where task/session status is persisted.
- [ ] Inspect logs for recent `engine: codex: exit signal: killed` events and capture timestamp/context.
- [ ] Determine whether Codex runs have an AgentHub timeout/deadline or are tied to request/WebSocket context cancellation.
- [ ] Add/centralize process exit classification for Codex subprocess errors, especially SIGKILL.
- [ ] Include stderr tail, runtime duration, engine name, and project/session/task metadata in error logs.
- [ ] Persist a clear terminal failure/cancelled message when Codex exits killed.
- [ ] Verify the UI/session does not leave the task looking unfinished after a killed Codex run.
- [ ] Add a unit/integration test using a fake command/process or injectable runner that simulates SIGKILL.
- [ ] Cross-check with `stream-codex-engine-events` so both changes share lifecycle/error handling cleanly.
