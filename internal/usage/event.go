package usage

// Event is a normalized token-usage record parsed from any source JSONL.
type Event struct {
	Source      string // "claude" | "codex"
	SessionID   string
	MessageID   string // Claude only; empty for Codex
	RequestID   string // Claude only; empty for Codex
	TS          int64  // unix epoch
	Model       string
	Input       int64
	Output      int64
	CacheCreate int64
	CacheRead   int64
	CostUSD     float64
	RawPath     string // source file path
}
