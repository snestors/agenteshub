package realtime

// RealtimeUsage holds the current usage snapshot for a single provider.
type RealtimeUsage struct {
	Source    string           `json:"source"`              // "claude" | "codex"
	Account   string           `json:"account,omitempty"`   // email
	Plan      string           `json:"plan,omitempty"`      // e.g. "max", "plus", "pro"
	Session   *RealtimeWindow  `json:"session,omitempty"`   // 5h window (primary)
	Weekly    *RealtimeWindow  `json:"weekly,omitempty"`    // 7d window (secondary)
	Credits   *RealtimeCredits `json:"credits,omitempty"`   // Codex credits balance
	FetchedAt int64            `json:"fetched_at"`          // unix epoch
	StaleAfter int64           `json:"stale_after"`         // unix epoch; when the cache entry expires
	Error     string           `json:"error,omitempty"`
}

// RealtimeWindow describes one rate-limit window.
type RealtimeWindow struct {
	PercentUsed     float64 `json:"percent_used"`
	WindowMins      int64   `json:"window_mins"`
	ResetAt         int64   `json:"reset_at"`   // unix epoch
}

// RealtimeCredits describes the balance of purchased credits (Codex).
type RealtimeCredits struct {
	HasCredits bool   `json:"has_credits"`
	Unlimited  bool   `json:"unlimited"`
	Balance    string `json:"balance"` // string as returned by the API
}
