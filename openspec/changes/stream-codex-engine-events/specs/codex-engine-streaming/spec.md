# Capability: codex-engine-streaming

## Requirements

### Requirement: Codex sessions stream live events
When a Codex-backed AgentHub run is started with `OnEvent`, the system SHALL emit progress events before the process exits.

#### Scenario: Codex produces assistant text
- WHEN `codex exec --json` emits an assistant message event
- THEN AgentHub forwards a `StreamEvent{Kind: "text"}` over the existing WebSocket topic.

### Requirement: Non-streaming behavior is preserved
When `OnEvent` is nil, CodexEngine SHALL keep returning a final `Result` without requiring WebSocket streaming.

#### Scenario: Background Codex run
- GIVEN a caller invokes CodexEngine without `OnEvent`
- WHEN the command completes
- THEN the final text, session id and token usage are parsed as before.

### Requirement: Engine IDs are not mixed
AgentHub SHALL NOT attempt to resume a Claude session id using Codex or a Codex session id using Claude.

#### Scenario: Engine mismatch
- GIVEN a project session has `engine=claude`
- WHEN the user selects Codex for a new task
- THEN AgentHub should create/use a Codex session id rather than reusing the Claude resume id.
