# Tasks

- [ ] Capture representative Codex `--json` event shapes from local CLI output or existing session JSONL.
- [ ] Add a streaming path in `internal/cliengine/codex.go` gated by `opts.OnEvent != nil`.
- [ ] Map Codex events to `cliengine.StreamEvent` kinds: `system`, `text`, `tool_use`, `tool_result`, `final` and error cases.
- [ ] Keep existing buffered parser for non-streaming calls.
- [ ] Ensure session/thread id and token usage still populate `Result`.
- [ ] Verify project session WebSocket receives live Codex events.
- [ ] Add parser tests/fixtures if feasible.
