package usage

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// claudeEntry is the minimal shape we decode from Claude JSONL lines.
type claudeEntry struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"` // RFC3339
	SessionID string `json:"sessionId"`
	RequestID string `json:"requestId"`
	Message   struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// ParseClaudeJSONL parses a Claude session JSONL file and returns usage Events.
// Only assistant entries with real (non-synthetic, non-zero) usage are returned.
// Dedup is handled at DB insert via the unique index — parser returns all valid events.
func ParseClaudeJSONL(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open claude jsonl %q: %w", path, err)
	}
	defer f.Close()

	var out []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e claudeEntry
		if err := json.Unmarshal(line, &e); err != nil {
			// Malformed line — skip silently, don't abort the whole file.
			continue
		}
		if e.Type != "assistant" {
			continue
		}
		// Skip synthetic placeholders.
		if e.Message.Model == "<synthetic>" || e.Message.Model == "" {
			continue
		}
		// Skip zero-usage lines (rate-limit errors, etc.).
		u := e.Message.Usage
		if u.InputTokens == 0 && u.OutputTokens == 0 &&
			u.CacheCreationInputTokens == 0 && u.CacheReadInputTokens == 0 {
			continue
		}
		// Parse timestamp — fall back to now if malformed.
		ts := time.Now().Unix()
		if e.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
				ts = t.Unix()
			} else if t, err := time.Parse(time.RFC3339Nano, e.Timestamp); err == nil {
				ts = t.Unix()
			}
		}
		out = append(out, Event{
			Source:      "claude",
			SessionID:   e.SessionID,
			MessageID:   e.Message.ID,
			RequestID:   e.RequestID,
			TS:          ts,
			Model:       e.Message.Model,
			Input:       u.InputTokens,
			Output:      u.OutputTokens,
			CacheCreate: u.CacheCreationInputTokens,
			CacheRead:   u.CacheReadInputTokens,
			RawPath:     path,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan claude jsonl %q: %w", path, err)
	}
	return out, nil
}
