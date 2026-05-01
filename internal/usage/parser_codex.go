package usage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// codexLine is the common wrapper for every line in a Codex rollout JSONL.
type codexLine struct {
	Timestamp string          `json:"timestamp"` // RFC3339
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

// codexTokenCountPayload is the shape when type=="event_msg" and payload.type=="token_count".
type codexTokenCountPayload struct {
	Type string `json:"type"`
	Info *struct {
		LastTokenUsage struct {
			InputTokens          int64 `json:"input_tokens"`
			CachedInputTokens    int64 `json:"cached_input_tokens"`
			OutputTokens         int64 `json:"output_tokens"`
			ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
		} `json:"last_token_usage"`
	} `json:"info"`
}

// codexTurnContextPayload carries the model per turn.
type codexTurnContextPayload struct {
	TurnID string `json:"turn_id"`
	Model  string `json:"model"`
}

// codexSessionMetaPayload carries the session id.
type codexSessionMetaPayload struct {
	ID string `json:"id"`
}

// ParseCodexJSONL parses a Codex rollout JSONL file and returns usage Events.
// Each token_count event with info.last_token_usage populated is one Event.
// The model is taken from the most recent turn_context seen before the event.
// Dedup is handled at DB insert via the unique index on (source, session_id, ts, input, output).
func ParseCodexJSONL(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open codex jsonl %q: %w", path, err)
	}
	defer f.Close()

	var out []Event
	sessionID := ""
	currentModel := "gpt-5.5" // fallback

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var l codexLine
		if err := json.Unmarshal(line, &l); err != nil {
			continue
		}

		switch l.Type {
		case "session_meta":
			var p codexSessionMetaPayload
			if err := json.Unmarshal(l.Payload, &p); err == nil && p.ID != "" {
				sessionID = p.ID
			}

		case "turn_context":
			var p codexTurnContextPayload
			if err := json.Unmarshal(l.Payload, &p); err == nil && p.Model != "" {
				currentModel = p.Model
			}

		case "event_msg":
			var p codexTokenCountPayload
			if err := json.Unmarshal(l.Payload, &p); err != nil {
				continue
			}
			if p.Type != "token_count" || p.Info == nil {
				continue
			}
			u := p.Info.LastTokenUsage
			// Skip zero-delta entries (events that fired before any model call).
			if u.InputTokens == 0 && u.OutputTokens == 0 && u.CachedInputTokens == 0 {
				continue
			}
			ts := time.Now().Unix()
			if l.Timestamp != "" {
				if t, err := time.Parse(time.RFC3339, l.Timestamp); err == nil {
					ts = t.Unix()
				} else if t, err := time.Parse(time.RFC3339Nano, l.Timestamp); err == nil {
					ts = t.Unix()
				}
			}
			sid := sessionID
			if sid == "" {
				// Derive from filename as last-resort (rollout-<date>T<time>-<uuid>.jsonl)
				sid = path
			}
			out = append(out, Event{
				Source:      "codex",
				SessionID:   sid,
				TS:          ts,
				Model:       currentModel,
				Input:       u.InputTokens,
				Output:      u.OutputTokens,
				CacheRead:   u.CachedInputTokens,
				RawPath:     path,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan codex jsonl %q: %w", path, err)
	}
	return out, nil
}
