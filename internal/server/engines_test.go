package server

import "testing"

// f-004: Codex is a tool of Claude, not a primary session engine. Any new
// callsite that picks the engine of a session has to gate on this — these
// tests pin the contract so a regression shows up immediately.

func TestValidPrimaryEngine_ClaudeAccepted(t *testing.T) {
	if !validPrimaryEngine("claude") {
		t.Errorf("validPrimaryEngine(claude) = false, want true")
	}
}

func TestValidPrimaryEngine_CodexRejected(t *testing.T) {
	if validPrimaryEngine("codex") {
		t.Errorf("validPrimaryEngine(codex) = true, want false (f-004)")
	}
}

func TestValidPrimaryEngine_UnknownRejected(t *testing.T) {
	for _, eng := range []string{"", "ollama", "claude-extended", "codex2"} {
		if validPrimaryEngine(eng) {
			t.Errorf("validPrimaryEngine(%q) = true, want false", eng)
		}
	}
}

func TestValidPrimaryEngineModel_ClaudeKnown(t *testing.T) {
	cases := []string{"sonnet", "opus", "haiku"}
	for _, m := range cases {
		if !validPrimaryEngineModel("claude", m) {
			t.Errorf("validPrimaryEngineModel(claude, %q) = false, want true", m)
		}
	}
}

func TestValidPrimaryEngineModel_CodexAlwaysRejected(t *testing.T) {
	// Even with a model that codex itself supports — primary engine has to be
	// Claude, period.
	for _, m := range []string{"gpt-5.5", "gpt-5.4", "glm-5.1"} {
		if validPrimaryEngineModel("codex", m) {
			t.Errorf("validPrimaryEngineModel(codex, %q) = true, want false (f-004)", m)
		}
	}
}

func TestValidPrimaryEngineModel_ClaudeUnknownModel(t *testing.T) {
	if validPrimaryEngineModel("claude", "gpt-5.5") {
		t.Errorf("claude with non-claude model should be rejected")
	}
}

// validEngine still accepts both because /api/agent/engines exposes both for
// the frontend to render. The gate is in validPrimaryEngine, not the
// catalogue.
func TestValidEngine_ListingStillCatalogsCodex(t *testing.T) {
	if !validEngine("codex") {
		t.Errorf("validEngine(codex) = false; the catalogue should still list it for /api/agent/engines")
	}
	if !validEngine("claude") {
		t.Errorf("validEngine(claude) = false")
	}
}
