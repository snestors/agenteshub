package cliengine

// Role identifies the kind of work an agent does. The (engine, model) tuple
// for a role is the policy frozen in ROADMAP.md "Decisiones clave":
//
//   orchestrator | meta  → claude opus    (cracks for planning + delegation)
//   verifier            → claude sonnet  (sound judgment, cheaper than opus)
//   executor            → codex gpt-5.5  (mechanical work, off the Anthropic budget)
//   watcher             → ollama gemma4:e2b (cheap recurring tasks, local)
//
// Agents may override these defaults declaratively (frontmatter `engine` /
// `model`) but the override must come with an explicit `engine_override_reason`
// in the frontmatter — the agent-builder enforces that contract.
const (
	RoleOrchestrator = "orchestrator"
	RoleMeta         = "meta"
	RoleVerifier     = "verifier"
	RoleExecutor     = "executor"
	RoleWatcher      = "watcher"
)

// RoleDefault returns the (engine, model) tuple that an agent of the given
// role should run on, when the agent doesn't override explicitly. Unknown
// roles fall back to claude sonnet — sane default that won't blow the budget.
func RoleDefault(role string) (engine, model string) {
	switch role {
	case RoleOrchestrator, RoleMeta:
		return "claude", "opus"
	case RoleVerifier:
		return "claude", "sonnet"
	case RoleExecutor:
		return "codex", "gpt-5.5"
	case RoleWatcher:
		return "ollama", "gemma4:e2b"
	default:
		return "claude", "sonnet"
	}
}

// ResolveEngineModel picks the engine + model to run a task with, given an
// optional explicit pair (from the agent row or RunOpts) and a role hint.
// Explicit values win; missing fields are filled from the role default.
func ResolveEngineModel(explicitEngine, explicitModel, role string) (string, string) {
	defEngine, defModel := RoleDefault(role)
	engine := explicitEngine
	model := explicitModel
	if engine == "" {
		engine = defEngine
	}
	if model == "" {
		model = defModel
	}
	return engine, model
}
