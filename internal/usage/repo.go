package usage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// UsageRepo handles persistence for usage_events.
type UsageRepo struct {
	db *sql.DB
}

// NewUsageRepo constructs a UsageRepo.
func NewUsageRepo(db *sql.DB) *UsageRepo { return &UsageRepo{db: db} }

// UpsertEvent inserts an event or ignores it if the unique constraint fires.
// Returns (true, nil) when inserted, (false, nil) when skipped.
func (r *UsageRepo) UpsertEvent(ctx context.Context, e Event) (inserted bool, err error) {
	now := time.Now().Unix()
	res, err := r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO usage_events
			(source, session_id, message_id, request_id, ts, model,
			 input_tokens, output_tokens, cache_create_tokens, cache_read_tokens,
			 cost_usd, raw_path, imported_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
	`,
		e.Source, e.SessionID,
		nullableStr(e.MessageID), nullableStr(e.RequestID),
		e.TS, e.Model,
		e.Input, e.Output, e.CacheCreate, e.CacheRead,
		e.CostUSD, e.RawPath, now,
	)
	if err != nil {
		return false, fmt.Errorf("upsert usage event: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func nullableStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// AggregateBucketRow is one row returned by AggregateBy.
type AggregateBucketRow struct {
	Key         string  `json:"key"`
	Events      int64   `json:"events"`
	Input       int64   `json:"input_tokens"`
	Output      int64   `json:"output_tokens"`
	CacheCreate int64   `json:"cache_create_tokens"`
	CacheRead   int64   `json:"cache_read_tokens"`
	CostUSD     float64 `json:"cost_usd"`
}

// UsageTotals aggregates the full result set.
type UsageTotals struct {
	Events      int64   `json:"events"`
	Input       int64   `json:"input_tokens"`
	Output      int64   `json:"output_tokens"`
	CacheCreate int64   `json:"cache_create_tokens"`
	CacheRead   int64   `json:"cache_read_tokens"`
	CostUSD     float64 `json:"cost_usd"`
}

// QueryParams holds all filter options for AggregateBy.
type QueryParams struct {
	Since   int64  // unix epoch, inclusive
	Until   int64  // unix epoch, inclusive
	Source  string // "claude" | "codex" | ""
	Model   string // exact match or ""
	GroupBy string // "day" | "model" | "source" | "session"
}

// AggregateBy runs a grouped aggregation returning totals + buckets.
func (r *UsageRepo) AggregateBy(ctx context.Context, p QueryParams) (UsageTotals, []AggregateBucketRow, error) {
	// Build WHERE clause.
	var conds []string
	var args []interface{}
	conds = append(conds, "ts >= ?", "ts <= ?")
	args = append(args, p.Since, p.Until)
	if p.Source != "" {
		conds = append(conds, "source = ?")
		args = append(args, p.Source)
	}
	if p.Model != "" {
		conds = append(conds, "model = ?")
		args = append(args, p.Model)
	}
	where := "WHERE " + strings.Join(conds, " AND ")

	// Key expression for GROUP BY.
	var keyExpr string
	switch p.GroupBy {
	case "model":
		keyExpr = "model"
	case "source":
		keyExpr = "source"
	case "session":
		keyExpr = "session_id"
	default: // "day"
		keyExpr = "strftime('%Y-%m-%d', ts, 'unixepoch')"
	}

	bucketQ := fmt.Sprintf(`
		SELECT
			%s AS key,
			COUNT(*) AS events,
			SUM(input_tokens)        AS input_tokens,
			SUM(output_tokens)       AS output_tokens,
			SUM(cache_create_tokens) AS cache_create_tokens,
			SUM(cache_read_tokens)   AS cache_read_tokens,
			SUM(cost_usd)            AS cost_usd
		FROM usage_events
		%s
		GROUP BY key
		ORDER BY key ASC
	`, keyExpr, where)

	rows, err := r.db.QueryContext(ctx, bucketQ, args...)
	if err != nil {
		return UsageTotals{}, nil, fmt.Errorf("usage aggregate query: %w", err)
	}
	defer rows.Close()

	var buckets []AggregateBucketRow
	var totals UsageTotals
	for rows.Next() {
		var b AggregateBucketRow
		if err := rows.Scan(&b.Key, &b.Events, &b.Input, &b.Output, &b.CacheCreate, &b.CacheRead, &b.CostUSD); err != nil {
			return UsageTotals{}, nil, fmt.Errorf("usage aggregate scan: %w", err)
		}
		buckets = append(buckets, b)
		totals.Events += b.Events
		totals.Input += b.Input
		totals.Output += b.Output
		totals.CacheCreate += b.CacheCreate
		totals.CacheRead += b.CacheRead
		totals.CostUSD += b.CostUSD
	}
	if err := rows.Err(); err != nil {
		return UsageTotals{}, nil, fmt.Errorf("usage aggregate rows: %w", err)
	}
	if buckets == nil {
		buckets = []AggregateBucketRow{}
	}
	return totals, buckets, nil
}

// Count returns the total number of rows in usage_events.
func (r *UsageRepo) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usage_events`).Scan(&n)
	return n, err
}

// FirstLast returns the earliest and latest ts in the table (0,0 if empty).
func (r *UsageRepo) FirstLast(ctx context.Context) (first, last int64, err error) {
	row := r.db.QueryRowContext(ctx, `SELECT COALESCE(MIN(ts),0), COALESCE(MAX(ts),0) FROM usage_events`)
	err = row.Scan(&first, &last)
	return
}
