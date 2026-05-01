package realtime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

var codexAuthFile = os.ExpandEnv("$HOME/.codex/auth.json")

// codexAuthJSON holds the subset of ~/.codex/auth.json we need.
type codexAuthJSON struct {
	AuthMode string `json:"auth_mode"`
	Tokens   struct {
		AccessToken string `json:"access_token"`
	} `json:"tokens"`
}

// rpc wire types -------------------------------------------------------

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type rpcResponse struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
	Method string          `json:"method"` // set for notifications (no ID)
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// accountResult mirrors account/read result.
type accountResult struct {
	Account struct {
		Type     string `json:"type"`
		Email    string `json:"email"`
		PlanType string `json:"planType"`
	} `json:"account"`
}

// rateLimitsResult mirrors account/rateLimits/read result.
type rateLimitsResult struct {
	RateLimits struct {
		Primary struct {
			UsedPercent      float64 `json:"usedPercent"`
			WindowDurationMins int64  `json:"windowDurationMins"`
			ResetsAt         int64   `json:"resetsAt"` // unix epoch seconds
		} `json:"primary"`
		Secondary struct {
			UsedPercent      float64 `json:"usedPercent"`
			WindowDurationMins int64  `json:"windowDurationMins"`
			ResetsAt         int64   `json:"resetsAt"`
		} `json:"secondary"`
		Credits struct {
			HasCredits bool   `json:"hasCredits"`
			Unlimited  bool   `json:"unlimited"`
			Balance    string `json:"balance"`
		} `json:"credits"`
		PlanType string `json:"planType"`
	} `json:"rateLimits"`
}

// FetchCodexUsage fetches the current Codex rate-limit windows via JSON-RPC.
// Uses 60-second in-memory cache; on fetch error returns previous valid value
// with Error field set.
func FetchCodexUsage(ctx context.Context, cache *Cache) (*RealtimeUsage, error) {
	const key = "codex"
	prev, valid := cache.Get(key)
	if valid {
		return prev, nil
	}

	result, err := fetchCodexDirect(ctx)
	if err != nil {
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

func fetchCodexDirect(ctx context.Context) (*RealtimeUsage, error) {
	// Total timeout: 8 seconds across the whole RPC session.
	rpcCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(rpcCtx, "codex", "-s", "read-only", "-a", "untrusted", "app-server")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("codex stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("codex stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("codex start: %w", err)
	}
	defer func() {
		stdin.Close()
		_ = cmd.Wait()
	}()

	reader := bufio.NewReader(stdout)

	send := func(req rpcRequest) error {
		data, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("marshal rpc request: %w", err)
		}
		data = append(data, '\n')
		_, err = io.Copy(stdin, bytes.NewReader(data))
		return err
	}

	// readUntilID reads lines, skipping notifications, until it finds the
	// response matching targetID.
	readUntilID := func(targetID int) (*rpcResponse, error) {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("codex read: %w", err)
			}
			trimmed := bytes.TrimSpace([]byte(line))
			if len(trimmed) == 0 {
				continue
			}
			var msg rpcResponse
			if err := json.Unmarshal(trimmed, &msg); err != nil {
				// Skip unparseable lines (e.g. startup noise).
				continue
			}
			// Notifications have no id (or id=0) and have Method set.
			if msg.Method != "" {
				continue
			}
			if msg.ID == targetID {
				return &msg, nil
			}
		}
	}

	// 1. initialize
	if err := send(rpcRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  map[string]any{"clientInfo": map[string]any{"name": "agenthub-usage", "version": "0.2.64"}},
	}); err != nil {
		return nil, fmt.Errorf("codex send initialize: %w", err)
	}
	if _, err := readUntilID(1); err != nil {
		return nil, fmt.Errorf("codex initialize response: %w", err)
	}

	// 2. account/read
	if err := send(rpcRequest{JSONRPC: "2.0", ID: 2, Method: "account/read", Params: map[string]any{}}); err != nil {
		return nil, fmt.Errorf("codex send account/read: %w", err)
	}
	accResp, err := readUntilID(2)
	if err != nil {
		return nil, fmt.Errorf("codex account/read response: %w", err)
	}
	if accResp.Error != nil {
		return nil, fmt.Errorf("codex account/read error %d: %s", accResp.Error.Code, accResp.Error.Message)
	}
	var acc accountResult
	if err := json.Unmarshal(accResp.Result, &acc); err != nil {
		return nil, fmt.Errorf("codex account/read decode: %w", err)
	}

	// 3. account/rateLimits/read
	if err := send(rpcRequest{JSONRPC: "2.0", ID: 3, Method: "account/rateLimits/read", Params: map[string]any{}}); err != nil {
		return nil, fmt.Errorf("codex send account/rateLimits/read: %w", err)
	}
	rlResp, err := readUntilID(3)
	if err != nil {
		return nil, fmt.Errorf("codex account/rateLimits/read response: %w", err)
	}
	if rlResp.Error != nil {
		return nil, fmt.Errorf("codex account/rateLimits/read error %d: %s", rlResp.Error.Code, rlResp.Error.Message)
	}
	var rl rateLimitsResult
	if err := json.Unmarshal(rlResp.Result, &rl); err != nil {
		return nil, fmt.Errorf("codex account/rateLimits/read decode: %w", err)
	}

	now := time.Now()
	result := &RealtimeUsage{
		Source:     "codex",
		Account:    acc.Account.Email,
		Plan:       acc.Account.PlanType,
		FetchedAt:  now.Unix(),
		StaleAfter: now.Add(cacheTTL).Unix(),
	}

	p := rl.RateLimits.Primary
	if p.WindowDurationMins > 0 {
		result.Session = &RealtimeWindow{
			PercentUsed: p.UsedPercent,
			WindowMins:  p.WindowDurationMins,
			ResetAt:     p.ResetsAt,
		}
	}

	s := rl.RateLimits.Secondary
	if s.WindowDurationMins > 0 {
		result.Weekly = &RealtimeWindow{
			PercentUsed: s.UsedPercent,
			WindowMins:  s.WindowDurationMins,
			ResetAt:     s.ResetsAt,
		}
	}

	result.Credits = &RealtimeCredits{
		HasCredits: rl.RateLimits.Credits.HasCredits,
		Unlimited:  rl.RateLimits.Credits.Unlimited,
		Balance:    rl.RateLimits.Credits.Balance,
	}

	return result, nil
}
