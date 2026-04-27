# Codex engine reliability delta

## ADDED Requirements

### Requirement: Codex killed exits SHALL be classified

When a Codex subprocess exits due to SIGKILL or equivalent `signal: killed`, AgentHub SHALL classify the failure separately from generic engine errors.

#### Scenario: Codex process is killed

- **WHEN** a Codex-backed task process exits with `signal: killed`
- **THEN** AgentHub records a terminal task/session failure
- **AND** the persisted message identifies the engine as Codex
- **AND** the error includes whether the kill is known timeout/cancel or unknown external kill when determinable.

### Requirement: Codex failures SHALL not leave tasks ambiguous

A Codex engine failure SHALL result in a visible terminal state/message for the user.

#### Scenario: Codex fails before final assistant output

- **WHEN** Codex exits before producing a final result
- **THEN** AgentHub persists an assistant/system error message with actionable context
- **AND** the UI no longer appears to be waiting for an unfinished task.
