package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

const anthropicUsageURL = "https://api.anthropic.com/api/oauth/usage"

// credentialsFile is the default path to the Claude OAuth credentials.
var credentialsFile = os.ExpandEnv("$HOME/.claude/.credentials.json")

// anthropicCreds is the subset of the credentials file we need.
type anthropicCreds struct {
	ClaudeAiOauth struct {
		AccessToken      string `json:"accessToken"`
		ExpiresAt        int64  `json:"expiresAt"` // unix ms
		SubscriptionType string `json:"subscriptionType"`
	} `json:"claudeAiOauth"`
}

// anthropicUsageResponse matches the real API response shape.
// The API returns utilization (0-100) and a reset timestamp; it does NOT
// expose raw token counts.
type anthropicUsageResponse struct {
	FiveHour *struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"` // RFC3339
	} `json:"five_hour"`
	SevenDay *struct {
		Utilization float64 `json:"utilization"`
		ResetsAt    string  `json:"resets_at"`
	} `json:"seven_day"`
}

// FetchAnthropicUsage fetches the current Claude rate-limit windows.
// Uses 60-second in-memory cache; on fetch error returns previous valid value
// with Error field set.
func FetchAnthropicUsage(ctx context.Context, cache *Cache) (*RealtimeUsage, error) {
	const key = "claude"
	prev, valid := cache.Get(key)
	if valid {
		return prev, nil
	}

	result, err := fetchAnthropicDirect(ctx)
	if err != nil {
		// Surface previous value + error rather than dropping stale data.
		if prev != nil {
			stale := *prev
			stale.Error = err.Error()
			return &stale, err
		}
		return &RealtimeUsage{
			Source:     key,
			FetchedAt:  time.Now().Unix(),
			StaleAfter: time.Now().Add(cacheTTL).Unix(),
			Error:      err.Error(),
		}, err
	}

	cache.Set(key, result)
	return result, nil
}

func fetchAnthropicDirect(ctx context.Context) (*RealtimeUsage, error) {
	creds, err := loadAnthropicCreds()
	if err != nil {
		return nil, fmt.Errorf("anthropic creds: %w", err)
	}

	// Check expiry (ms epoch).
	if creds.ClaudeAiOauth.ExpiresAt > 0 {
		expiresAt := time.UnixMilli(creds.ClaudeAiOauth.ExpiresAt)
		if time.Now().After(expiresAt) {
			return nil, fmt.Errorf("anthropic token expired at %s — re-login with claude CLI", expiresAt.Format(time.RFC3339))
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, anthropicUsageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("anthropic build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+creds.ClaudeAiOauth.AccessToken)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic usage fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic usage: unexpected status %d", resp.StatusCode)
	}

	var raw anthropicUsageResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("anthropic usage decode: %w", err)
	}

	now := time.Now()
	result := &RealtimeUsage{
		Source:     "claude",
		Account:    "",            // not returned by this endpoint
		Plan:       creds.ClaudeAiOauth.SubscriptionType,
		FetchedAt:  now.Unix(),
		StaleAfter: now.Add(cacheTTL).Unix(),
	}

	if raw.FiveHour != nil {
		w := &RealtimeWindow{
			PercentUsed: raw.FiveHour.Utilization,
			WindowMins:  300,
		}
		if t, err := time.Parse(time.RFC3339Nano, raw.FiveHour.ResetsAt); err == nil {
			w.ResetAt = t.Unix()
		}
		result.Session = w
	}

	if raw.SevenDay != nil {
		w := &RealtimeWindow{
			PercentUsed: raw.SevenDay.Utilization,
			WindowMins:  7 * 24 * 60,
		}
		if t, err := time.Parse(time.RFC3339Nano, raw.SevenDay.ResetsAt); err == nil {
			w.ResetAt = t.Unix()
		}
		result.Weekly = w
	}

	return result, nil
}

func loadAnthropicCreds() (*anthropicCreds, error) {
	data, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", credentialsFile, err)
	}
	var c anthropicCreds
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	if c.ClaudeAiOauth.AccessToken == "" {
		return nil, fmt.Errorf("claudeAiOauth.accessToken is empty")
	}
	return &c, nil
}
