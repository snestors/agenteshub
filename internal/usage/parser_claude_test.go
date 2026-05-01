package usage

import (
	"os"
	"testing"
)

// TestParseClaudeJSONL_dedup verifica que reimportar el mismo archivo no produzca
// eventos duplicados: el unique index del DB hace el trabajo final, pero el parser
// debe emitir los duplicates de streaming para que el INSERT OR IGNORE los absorba.
// Aquí verificamos que el parser los devuelve y que en la práctica, dos llamadas
// seguidas con el mismo archivo producen los mismos message_id/request_id.
func TestParseClaudeJSONL_dedup(t *testing.T) {
	// Fixture: dos líneas con el mismo message_id y request_id (streaming chunks reales)
	// más una línea sintética que debe filtrarse, y una con zero tokens que también se filtra.
	fixture := `{"type":"assistant","timestamp":"2026-04-30T10:00:00.000Z","sessionId":"sess-abc","requestId":"req_AAA","message":{"id":"msg_AAA","model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":200,"cache_read_input_tokens":0}}}
{"type":"assistant","timestamp":"2026-04-30T10:00:00.100Z","sessionId":"sess-abc","requestId":"req_AAA","message":{"id":"msg_AAA","model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":50,"cache_creation_input_tokens":200,"cache_read_input_tokens":0}}}
{"type":"assistant","timestamp":"2026-04-30T10:01:00.000Z","sessionId":"sess-abc","requestId":"req_BBB","message":{"id":"msg_BBB","model":"<synthetic>","usage":{"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}
{"type":"assistant","timestamp":"2026-04-30T10:02:00.000Z","sessionId":"sess-abc","requestId":"req_CCC","message":{"id":"msg_CCC","model":"claude-sonnet-4-6","usage":{"input_tokens":0,"output_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}
{"type":"assistant","timestamp":"2026-04-30T10:03:00.000Z","sessionId":"sess-abc","requestId":"req_DDD","message":{"id":"msg_DDD","model":"claude-sonnet-4-6","usage":{"input_tokens":300,"output_tokens":80,"cache_creation_input_tokens":0,"cache_read_input_tokens":500}}}
{"type":"user","timestamp":"2026-04-30T10:04:00.000Z","sessionId":"sess-abc"}
`
	f, err := os.CreateTemp("", "claude-test-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(fixture); err != nil {
		t.Fatal(err)
	}
	f.Close()

	events1, err := ParseClaudeJSONL(f.Name())
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	// Expected: msg_AAA appears twice (streaming dup), msg_DDD once. Synthetic and zero filtered.
	if len(events1) != 3 {
		t.Fatalf("expected 3 events (2 streaming dups + 1 real), got %d", len(events1))
	}

	// Verify the two streaming duplicates have identical (message_id, request_id).
	if events1[0].MessageID != "msg_AAA" || events1[1].MessageID != "msg_AAA" {
		t.Errorf("expected two msg_AAA entries, got %q and %q", events1[0].MessageID, events1[1].MessageID)
	}
	if events1[0].RequestID != events1[1].RequestID {
		t.Errorf("streaming dups should share request_id, got %q vs %q", events1[0].RequestID, events1[1].RequestID)
	}

	// Second parse returns the same events — reimport is idempotent at DB level.
	events2, err := ParseClaudeJSONL(f.Name())
	if err != nil {
		t.Fatalf("second parse: %v", err)
	}
	if len(events1) != len(events2) {
		t.Errorf("second parse: expected %d events, got %d", len(events1), len(events2))
	}
	for i := range events1 {
		if events1[i].MessageID != events2[i].MessageID || events1[i].RequestID != events2[i].RequestID {
			t.Errorf("event %d differs between parses", i)
		}
	}

	// Verify model and token fields on the last (unique) event.
	last := events1[2]
	if last.Model != "claude-sonnet-4-6" {
		t.Errorf("expected model claude-sonnet-4-6, got %q", last.Model)
	}
	if last.Input != 300 || last.Output != 80 || last.CacheRead != 500 {
		t.Errorf("unexpected token counts: input=%d output=%d cache_read=%d", last.Input, last.Output, last.CacheRead)
	}
}

// TestParseClaudeJSONL_emptyFile checks that parsing an empty file returns no error.
func TestParseClaudeJSONL_emptyFile(t *testing.T) {
	f, err := os.CreateTemp("", "claude-empty-*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	events, err := ParseClaudeJSONL(f.Name())
	if err != nil {
		t.Fatalf("empty file should not error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}
