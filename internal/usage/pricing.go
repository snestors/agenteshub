// Package usage provides token-usage tracking for Claude and Codex sessions.
package usage

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// ModelPricing holds cost per 1M tokens in USD for a single model.
type ModelPricing struct {
	InputPerM       float64 `json:"input_per_m"`
	OutputPerM      float64 `json:"output_per_m"`
	CacheCreatePerM float64 `json:"cache_create_per_m"`
	CacheReadPerM   float64 `json:"cache_read_per_m"`
}

// defaultPricing is hardcoded as of April 2026.
// TODO: confirm latest pricing at https://www.anthropic.com/pricing and https://openai.com/api/pricing
var defaultPricing = map[string]ModelPricing{
	// Anthropic Claude
	"claude-opus-4-7":        {InputPerM: 15.00, OutputPerM: 75.00, CacheCreatePerM: 18.75, CacheReadPerM: 1.50},
	"claude-sonnet-4-6":      {InputPerM: 3.00, OutputPerM: 15.00, CacheCreatePerM: 3.75, CacheReadPerM: 0.30},
	"claude-sonnet-4-5":      {InputPerM: 3.00, OutputPerM: 15.00, CacheCreatePerM: 3.75, CacheReadPerM: 0.30},
	"claude-haiku-4-5":       {InputPerM: 0.80, OutputPerM: 4.00, CacheCreatePerM: 1.00, CacheReadPerM: 0.08},
	"claude-opus-4-5":        {InputPerM: 15.00, OutputPerM: 75.00, CacheCreatePerM: 18.75, CacheReadPerM: 1.50},
	"claude-3-7-sonnet-20250219": {InputPerM: 3.00, OutputPerM: 15.00, CacheCreatePerM: 3.75, CacheReadPerM: 0.30},
	"claude-3-5-haiku-20241022":  {InputPerM: 0.80, OutputPerM: 4.00, CacheCreatePerM: 1.00, CacheReadPerM: 0.08},
	// OpenAI Codex / GPT
	"gpt-5.5":   {InputPerM: 2.50, OutputPerM: 10.00},
	"gpt-4.1":   {InputPerM: 2.00, OutputPerM: 8.00},
	"gpt-4o":    {InputPerM: 5.00, OutputPerM: 15.00},
	"o3":        {InputPerM: 10.00, OutputPerM: 40.00},
	"o4-mini":   {InputPerM: 1.10, OutputPerM: 4.40},
}

// PricingTable merges defaults with optional overrides from data/pricing.json.
type PricingTable struct {
	models map[string]ModelPricing
	log    *slog.Logger
}

// LoadPricingTable builds a PricingTable, optionally merging data/pricing.json.
// Unknown models get cost=0 (warn, don't fail).
func LoadPricingTable(overridePath string, log *slog.Logger) *PricingTable {
	pt := &PricingTable{
		models: make(map[string]ModelPricing, len(defaultPricing)),
		log:    log,
	}
	for k, v := range defaultPricing {
		pt.models[k] = v
	}
	if overridePath != "" {
		if data, err := os.ReadFile(overridePath); err == nil {
			var overrides map[string]ModelPricing
			if err := json.Unmarshal(data, &overrides); err != nil {
				log.Warn("usage: pricing.json parse error", "err", err)
			} else {
				for k, v := range overrides {
					pt.models[k] = v
				}
				log.Info("usage: pricing overrides loaded", "path", overridePath, "count", len(overrides))
			}
		}
	}
	return pt
}

// Cost calculates the total USD cost for an event.
// Returns 0 and logs a warning if the model is not in the table.
func (pt *PricingTable) Cost(model string, inputTokens, outputTokens, cacheCreate, cacheRead int64) float64 {
	p, ok := pt.models[normalizeModel(model)]
	if !ok {
		pt.log.Warn("usage: unknown model, cost=0", "model", model)
		return 0
	}
	const perM = 1_000_000.0
	return float64(inputTokens)*p.InputPerM/perM +
		float64(outputTokens)*p.OutputPerM/perM +
		float64(cacheCreate)*p.CacheCreatePerM/perM +
		float64(cacheRead)*p.CacheReadPerM/perM
}

// normalizeModel strips common suffixes/prefixes to find the closest match.
// Tries exact first, then known aliases.
func normalizeModel(model string) string {
	model = strings.TrimSpace(strings.ToLower(model))
	if _, ok := defaultPricing[model]; ok {
		return model
	}
	// Try prefix match for versioned names like "claude-sonnet-4-6-20250613"
	for k := range defaultPricing {
		if strings.HasPrefix(model, k) {
			return k
		}
	}
	return model
}

// Validate returns an error if pricing is obviously broken.
func (pt *PricingTable) Validate() error {
	if len(pt.models) == 0 {
		return fmt.Errorf("pricing table is empty")
	}
	return nil
}
