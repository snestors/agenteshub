# Proposal: Fix CodexEngine killed task exits

## Problem

AgentHub project/agent tasks using the Codex engine can fail with:

```text
engine: codex: exit signal: killed
```

When this happens, the task does not finish cleanly and the user only sees a low-context engine failure. The system should distinguish intentional cancellation, timeout/resource kill, CLI crash, and parent-context shutdown, then persist a useful final state/message.

## Goal

Make Codex-backed tasks terminate predictably: either complete with a final result or fail with a classified, actionable error that is persisted and surfaced in the UI/session history.

## Scope

- Investigate where `signal: killed` originates for Codex runs: context cancellation, timeout, OOM killer, manual kill, service restart, or CLI subprocess behavior.
- Audit `internal/cliengine/codex.go` process lifecycle: context, command start/run/wait, stdout/stderr handling, and final error wrapping.
- Audit project/agent task orchestration around Codex: cancellation, timeouts, WebSocket disconnect handling, and persistence of failed task states.
- Add error classification for killed/interrupted Codex processes.
- Persist a final failure message/status so tasks do not remain ambiguous or appear unfinished.
- Add logs with enough metadata to debug: engine, project/session/task ids, duration, signal, stderr tail, and timeout/cancel reason where available.

## Non-goals

- Do not implement the Codex streaming feature here; that is tracked separately by `stream-codex-engine-events`.
- Do not change ClaudeEngine behavior except for shared helpers if strictly necessary.
- Do not mask real failures as success.

## Acceptance criteria

- Reproducing a killed Codex subprocess produces a classified error in logs and persisted session/task output.
- UI/session history shows a clear final failure message instead of an indefinite/unclear unfinished state.
- If the kill was caused by AgentHub timeout/cancellation, the message explicitly says so.
- If the kill reason is external/unknown, logs include enough context to investigate without rerunning blind.
