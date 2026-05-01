package usage

import (
	"os"
	"testing"
)

// TestParseCodexJSONL_dedup verifica que reimportar el mismo archivo no genere
// eventos con información distinta — el parser es determinista y el DB absorbe
// los duplicates via INSERT OR IGNORE.
func TestParseCodexJSONL_dedup(t *testing.T) {
	fixture := `{"timestamp":"2026-04-29T03:43:03.268Z","type":"session_meta","payload":{"id":"019dd755-0ba6-7950-9942-1888d0519e97","timestamp":"2026-04-29T03:42:48.246Z","cwd":"/home/nestor/agenthub"}}
{"timestamp":"2026-04-29T03:43:03.270Z","type":"turn_context","payload":{"turn_id":"019dd755-0bbf-7cf2-85a4-14c564c6e808","model":"gpt-5.5","cwd":"/home/nestor/agenthub"}}
{"timestamp":"2026-04-29T03:43:03.270Z","type":"event_msg","payload":{"type":"token_count","info":null}}
{"timestamp":"2026-04-29T03:43:15.944Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":14445,"cached_input_tokens":7552,"output_tokens":534,"reasoning_output_tokens":516,"total_tokens":14979},"total_token_usage":{"input_tokens":14445}}}}
{"timestamp":"2026-04-29T03:43:19.908Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":14996,"cached_input_tokens":7552,"output_tokens":130,"reasoning_output_tokens":15,"total_tokens":15126},"total_token_usage":{"input_tokens":29441}}}}
{"timestamp":"2026-04-29T03:43:23.466Z","type":"event_msg","payload":{"type":"user_message","payload":{}}}
`
	f, err := os.CreateTemp("", "codex-test-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(fixture); err != nil {
		t.Fatal(err)
	}
	f.Close()

	events1, err := ParseCodexJSONL(f.Name())
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	// Expected: 2 real token_count events (the null info one is skipped).
	if len(events1) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events1))
	}

	// Verify model comes from turn_context.
	if events1[0].Model != "gpt-5.5" {
		t.Errorf("expected model gpt-5.5, got %q", events1[0].Model)
	}

	// Verify token values on first event.
	if events1[0].Input != 14445 || events1[0].Output != 534 || events1[0].CacheRead != 7552 {
		t.Errorf("first event: input=%d output=%d cache_read=%d", events1[0].Input, events1[0].Output, events1[0].CacheRead)
	}

	// Second parse is deterministic.
	events2, err := ParseCodexJSONL(f.Name())
	if err != nil {
		t.Fatalf("second parse: %v", err)
	}
	if len(events1) != len(events2) {
		t.Errorf("second parse: expected %d events, got %d", len(events1), len(events2))
	}
	for i := range events1 {
		if events1[i].Input != events2[i].Input || events1[i].Output != events2[i].Output {
			t.Errorf("event %d differs between parses", i)
		}
	}

	// Session ID is extracted from session_meta.
	if events1[0].SessionID != "019dd755-0ba6-7950-9942-1888d0519e97" {
		t.Errorf("expected session id from session_meta, got %q", events1[0].SessionID)
	}
}

// TestParseCodexJSONL_emptyFile checks empty file produces no error.
func TestParseCodexJSONL_emptyFile(t *testing.T) {
	f, err := os.CreateTemp("", "codex-empty-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	events, err := ParseCodexJSONL(f.Name())
	if err != nil {
		t.Fatalf("empty file should not error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}
