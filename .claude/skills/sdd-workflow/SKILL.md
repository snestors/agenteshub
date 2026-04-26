---
name: sdd-workflow
description: >
  Project-local OpenSpec/SDD workflow rules for gated proposal, design, tasks, apply, verify, and archive phases.
  Trigger: When generating or reviewing OpenSpec artifacts, SDD proposals/design/tasks/spec deltas, or gated implementation plans.
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "1.0"
---

## When to Use

Use this skill when an AgentHub project change goes through OpenSpec/SDD:

- `sdd-propose` creates or regenerates `proposal.md`.
- `sdd-design` creates or regenerates `design.md` after proposal approval.
- `sdd-tasks` creates or regenerates `tasks.md` after design approval.
- `sdd-apply` executes only after tasks approval.
- `sdd-verify` validates implementation before archive.

## Critical Patterns

- NEVER execute implementation before explicit user approval of tasks.
- Each phase is gated: propose → APPROVE → design → APPROVE → tasks → APPROVE → apply → verify → APPROVE → archive.
- User feedback regenerates the current phase; it does not skip gates.
- Requirements in `openspec/specs/**/spec.md` and change deltas SHALL use `SHALL`.
- Acceptance criteria must be observable and testable. Avoid ambiguous words like “better”, “fast”, or “nice” unless quantified.
- Deltas live under `openspec/changes/<change-name>/specs/<capability>/spec.md`.
- Archive copies approved deltas into `openspec/specs/` and moves the change folder to `openspec/archive/`.

## Artifact Templates

### proposal.md

```markdown
## What

## Why

## Acceptance criteria

- [ ] ...

## Out of scope

- ...
```

### design.md

```markdown
## Context

## Decisions

## Architecture / files affected

## Data model / API changes

## Risks and mitigations

## Spec deltas to create/update
```

### tasks.md

```markdown
## Implementation tasks

- [ ] ...

## Verification

- [ ] ...

## Rollback

- [ ] ...
```

### spec.md

```markdown
# <capability> Specification

## Purpose

## Requirements

### Requirement: <name>

The system SHALL ...

#### Scenario: <name>

- GIVEN ...
- WHEN ...
- THEN ...
```

## Commands

```bash
# Backend smoke sequence (use dry_run to avoid real apply)
POST /api/projects/{id}/openspec/changes
POST /api/projects/{id}/openspec/changes/{name}/approve
POST /api/projects/{id}/openspec/changes/{name}/approve
POST /api/projects/{id}/openspec/changes/{name}/approve
POST /api/projects/{id}/openspec/changes/{name}/approve {"dry_run":true}
```
