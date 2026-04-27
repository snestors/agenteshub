# Design sketch

## Investigation path

1. Find all CodexEngine call sites and identify the lifecycle owner for each task/session.
2. Trace cancellation sources:
   - request context / WebSocket context
   - project session context
   - configured engine timeout, if any
   - service restart / parent process shutdown
   - OS kill / OOM killer
3. Reproduce with a controlled Codex command or fake process that exits via SIGKILL.

## Proposed implementation direction

- Introduce/extend a process error classifier near `internal/cliengine/codex.go`:
  - `context.Canceled`
  - `context.DeadlineExceeded`
  - `exec.ExitError` with `Signal() == SIGKILL`
  - stderr-only CLI failure
  - unknown process failure
- Preserve stderr tail and runtime duration in the wrapped error.
- Ensure orchestration stores a terminal assistant/error message when the engine returns an error.
- Ensure task/session status is terminal (`failed`, `cancelled`, or equivalent) instead of staying in a running/pending state.

## Relationship to Codex streaming

This change can be implemented before or after `stream-codex-engine-events`, but should not depend on streaming. If streaming lands first, reuse its process lifecycle path and add the same classifier around `Wait()`.

## Risks

- SIGKILL may be external and not fully attributable from Go. In that case, classify as `killed_unknown` and log host-level hints.
- Overly broad retry behavior could duplicate expensive Codex runs; do not auto-retry until the cause is known.
