package cliengine

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/snestors/agenteshub/internal/store"
)

// captureSubagents parses the JSONL of a Claude session and persists any
// sub-agent (Agent tool) calls it finds into subagent_runs.
//
// We re-process the whole JSONL each turn — it's cheap (one read) and idempotent
// because Start() inserts and we don't dedupe (good enough for v0; a future
// version can hash by tool_use_id).
func (m *Manager) captureSubagents(ctx context.Context, sessionID, jsonlPath string, opts RunOpts) {
	f, err := os.Open(jsonlPath)
	if err != nil {
		return
	}
	defer f.Close()

	type toolUse struct {
		Type  string          `json:"type"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	type entry struct {
		Type    string `json:"type"`
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !strings.Contains(string(line), `"Agent"`) {
			continue
		}
		var e entry
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		// Content can be a string or an array of blocks; only the array form
		// holds tool_use blocks.
		var blocks []toolUse
		if err := json.Unmarshal(e.Message.Content, &blocks); err != nil {
			continue
		}
		for _, b := range blocks {
			if b.Type != "tool_use" || b.Name != "Agent" {
				continue
			}
			var input struct {
				Description  string `json:"description"`
				Prompt       string `json:"prompt"`
				SubagentType string `json:"subagent_type"`
			}
			_ = json.Unmarshal(b.Input, &input)
			scope := opts.Scope
			if scope == "" {
				scope = "main"
			}
			_, _ = m.repos.Subagents.Start(ctx, store.SubagentRun{
				ParentSessionID: sessionID,
				ParentScope:     scope,
				AgentType:       sqlStr(input.SubagentType),
				Description:     sqlStr(input.Description),
				Prompt:          sqlStr(input.Prompt),
				Status:          "ok", // captured post-turn — we don't see them running, only completed
			})
		}
	}
}
