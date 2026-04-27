# Design sketch

## Current state

`internal/cliengine/codex.go` invokes:

```go
cmd.Stdout = &stdout
cmd.Stderr = &stderr
runErr := cmd.Run()
```

Then it scans stdout after completion. This prevents live UI streaming.

## Proposed approach

Mirror ClaudeEngine's streaming split:

- `Run()` keeps current buffered behavior when `opts.OnEvent == nil`.
- If `opts.OnEvent != nil`, call `runStreaming(ctx, cmd, opts)`.
- `runStreaming` uses `cmd.StdoutPipe()`, starts the process, scans JSONL line-by-line, emits `StreamEvent`s, aggregates final text/session id/token usage, then waits for process exit.

## Mapping

Codex JSONL should be normalized into the existing `StreamEvent` contract so the frontend does not need a new transport. Unknown event types can be emitted as `system` or ignored until fixtures define better mapping.

## Risk

Codex CLI event schema may differ from Claude. Keep parser permissive and preserve raw fallback text for unknown events.
