# Proposal: Stream CodexEngine events over WebSocket

## Problem

AgentHub already streams Claude turns live through `OnEvent`, `claude --output-format stream-json`, `StdoutPipe`, and WebSocket broadcasting. Codex, however, currently runs with `codex exec --json` but buffers stdout until the process exits, so the UI receives no live progress for Codex-backed project sessions or agents.

## Goal

Make `CodexEngine` support live event streaming equivalent to ClaudeEngine: read Codex JSONL events line-by-line, map useful events into `cliengine.StreamEvent`, and call `opts.OnEvent` while preserving the final `Result` behavior.

## Scope

- Update `internal/cliengine/codex.go` to use `StdoutPipe` when `opts.OnEvent != nil`.
- Parse `codex exec --json` JSONL incrementally.
- Emit WebSocket-friendly events for task start/progress, assistant text, tool usage/results, errors, and final result where available.
- Preserve existing non-streaming behavior for callers without `OnEvent`.
- Add tests or a small parser-level fixture if practical.

## Non-goals

- Do not change the Codex CLI itself.
- Do not change ClaudeEngine behavior.
- Do not mix Claude resume IDs with Codex resume IDs.
- Do not implement UI redesign beyond using the existing `StreamEvent` path.

## Notes

Local check on 2026-04-27 confirmed `codex exec --json` says it prints events to stdout as JSONL. The current AgentHub implementation still buffers because it uses `cmd.Run()` with `bytes.Buffer`.
