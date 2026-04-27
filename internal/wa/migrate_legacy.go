package wa

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// MigrateLegacyResult is the summary returned by MigrateLegacyMessages.
type MigrateLegacyResult struct {
	Total    int // rows seen in legacy DB
	Imported int // rows inserted into wa_messages
	Skipped  int // rows skipped because external_id already present (idempotent re-run)
	Errors   int // rows that failed to insert (non-fatal)
}

// MigrateLegacyMessages imports the message history from the legacy node
// bridge SQLite (default /home/nestor/mcp-whatsapp/data/messages.db) into the
// AgentHub wa_messages table. Each legacy row gets external_id="legacy:<id>"
// so re-running the migration is a no-op.
//
// The legacy schema is tiny:
//
//	messages(id, jid, phone, name, body, timestamp, from_me, is_read, created_at)
//
// We only carry the fields wa_messages cares about. Media is NOT migrated —
// the legacy bridge stored only paths in /home/nestor/mcp-whatsapp/data/media
// and we don't want to copy potentially large blobs without explicit ask.
func MigrateLegacyMessages(ctx context.Context, agentDB *sql.DB, legacyDBPath string) (*MigrateLegacyResult, error) {
	legacy, err := sql.Open("sqlite3", legacyDBPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open legacy db: %w", err)
	}
	defer legacy.Close()

	rows, err := legacy.QueryContext(ctx, `
		SELECT id, jid, body, timestamp, from_me, COALESCE(is_read, 0)
		FROM messages
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query legacy messages: %w", err)
	}
	defer rows.Close()

	res := &MigrateLegacyResult{}

	insertStmt, err := agentDB.PrepareContext(ctx, `
		INSERT INTO wa_messages(channel, direction, jid, body, ts, is_read, external_id)
		VALUES('wa', ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare insert: %w", err)
	}
	defer insertStmt.Close()

	checkStmt, err := agentDB.PrepareContext(ctx, `
		SELECT 1 FROM wa_messages WHERE external_id = ? LIMIT 1
	`)
	if err != nil {
		return nil, fmt.Errorf("prepare check: %w", err)
	}
	defer checkStmt.Close()

	for rows.Next() {
		res.Total++

		var id int64
		var jid, body, tsStr sql.NullString
		var fromMe, isRead int
		if err := rows.Scan(&id, &jid, &body, &tsStr, &fromMe, &isRead); err != nil {
			res.Errors++
			continue
		}

		extID := "legacy:" + strconv.FormatInt(id, 10)

		// Idempotent skip: already imported in a previous run.
		var dummy int
		if err := checkStmt.QueryRowContext(ctx, extID).Scan(&dummy); err == nil {
			res.Skipped++
			continue
		}

		// timestamp comes as RFC3339 with millis. Fall back to created_at by
		// using time.Now() if parsing fails (preserves the row instead of dropping it).
		var ts int64
		if tsStr.Valid {
			t, err := time.Parse(time.RFC3339, tsStr.String)
			if err != nil {
				// some legacy rows use 2006-01-02 15:04:05 form
				if t2, err2 := time.Parse("2006-01-02T15:04:05.000Z", tsStr.String); err2 == nil {
					ts = t2.Unix()
				} else if t3, err3 := time.Parse("2006-01-02 15:04:05", tsStr.String); err3 == nil {
					ts = t3.Unix()
				} else {
					ts = time.Now().Unix()
				}
			} else {
				ts = t.Unix()
			}
		} else {
			ts = time.Now().Unix()
		}

		direction := "in"
		if fromMe == 1 {
			direction = "out"
		}

		var bodyVal sql.NullString
		if body.Valid && body.String != "" {
			bodyVal = sql.NullString{String: body.String, Valid: true}
		}
		var jidVal sql.NullString
		if jid.Valid && jid.String != "" {
			jidVal = sql.NullString{String: jid.String, Valid: true}
		}

		if _, err := insertStmt.ExecContext(ctx, direction, jidVal, bodyVal, ts, isRead, extID); err != nil {
			res.Errors++
			continue
		}
		res.Imported++
	}
	if err := rows.Err(); err != nil {
		return res, fmt.Errorf("iterate legacy rows: %w", err)
	}
	return res, nil
}
