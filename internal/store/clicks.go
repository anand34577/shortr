package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// InsertClicksBatch appends events in one transaction and bumps click_count
// per affected link. Called by the async click writer, never on the redirect
// hot path.
func (s *Store) InsertClicksBatch(ctx context.Context, clicks []*Click, countBots bool) error {
	if len(clicks) == 0 {
		return nil
	}
	return s.WithTx(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, s.q(`INSERT INTO clicks (link_id, ts, ip, ip_version, country, region, city, referrer, referrer_host, user_agent, device, os, os_version, browser, browser_version, is_bot, lang, utm_source, utm_medium, utm_campaign, utm_term, utm_content, qs) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`))
		if err != nil {
			return err
		}
		defer stmt.Close()

		counts := map[string]int64{}
		lastTS := map[string]time.Time{}
		for _, c := range clicks {
			_, err := stmt.ExecContext(ctx, c.LinkID, toMillis(c.TS), c.IP, c.IPVersion, c.Country, c.Region, c.City,
				c.Referrer, c.ReferrerHost, c.UserAgent, c.Device, c.OS, c.OSVersion, c.Browser, c.BrowserVersion,
				boolToInt(c.IsBot), c.Lang, c.UTMSource, c.UTMMedium, c.UTMCampaign, c.UTMTerm, c.UTMContent, c.QS)
			if err != nil {
				return fmt.Errorf("insert click: %w", err)
			}
			if countBots || !c.IsBot {
				counts[c.LinkID]++
			}
			if lastTS[c.LinkID].Before(c.TS) {
				lastTS[c.LinkID] = c.TS
			}
		}
		for linkID, n := range counts {
			if _, err := s.txExec(tx, ctx, `UPDATE links SET click_count = click_count + ?, last_click_at = ? WHERE id = ?`, n, toMillis(lastTS[linkID]), linkID); err != nil {
				return err
			}
		}
		return nil
	})
}

// SpoolClicks and ReplaySpool live in click/writer.go (file-based, no store
// dependency needed there); this file only touches the DB.

type ClickFilter struct {
	LinkID string
	Cursor string
	Limit  int
}

// RecentActivityRow is one row of the dashboard's live activity feed — a
// click joined with its link's code (for display/linking), scoped to a
// user's own links unless userID is "" (admin "all" view).
type RecentActivityRow struct {
	ClickID      int64
	LinkID       string
	Code         string
	Country      string
	Device       string
	TS           time.Time
	ReferrerHost string
}

func (s *Store) ListRecentActivity(ctx context.Context, userID string, limit int) ([]RecentActivityRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	q := `SELECT c.id, c.link_id, l.code, c.country, c.device, c.ts, c.referrer_host
		FROM clicks c JOIN links l ON l.id = c.link_id
		WHERE c.is_bot = 0`
	args := []any{}
	if userID != "" {
		q += ` AND l.user_id = ?`
		args = append(args, userID)
	}
	q += ` ORDER BY c.ts DESC, c.id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RecentActivityRow
	for rows.Next() {
		var row RecentActivityRow
		var tsMS int64
		if err := rows.Scan(&row.ClickID, &row.LinkID, &row.Code, &row.Country, &row.Device, &tsMS, &row.ReferrerHost); err != nil {
			return nil, err
		}
		row.TS = fromMillis(tsMS)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) ListClicks(ctx context.Context, f ClickFilter) ([]*Click, string, error) {
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	where := []string{"link_id = ?"}
	args := []any{f.LinkID}
	if f.Cursor != "" {
		ms, _, err := decodeCursor(f.Cursor)
		if err != nil {
			return nil, "", err
		}
		where = append(where, "ts < ?")
		args = append(args, ms)
	}
	sqlStr := `SELECT id, link_id, ts, ip, ip_version, country, region, city, referrer, referrer_host, user_agent, device, os, os_version, browser, browser_version, is_bot, lang, utm_source, utm_medium, utm_campaign, utm_term, utm_content, qs FROM clicks WHERE ` + strings.Join(where, " AND ") + ` ORDER BY ts DESC, id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.query(ctx, sqlStr, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []*Click
	for rows.Next() {
		c, err := scanClick(rows)
		if err != nil {
			return nil, "", err
		}
		out = append(out, c)
	}
	next := ""
	if len(out) > limit {
		last := out[limit-1]
		next = encodeCursor(toMillis(last.TS), fmt.Sprint(last.ID))
		out = out[:limit]
	}
	return out, next, rows.Err()
}

// StreamClicks calls fn for every click matching linkID in ts order, for
// export. Uses a plain query (not ListClicks) to avoid buffering everything.
func (s *Store) StreamClicks(ctx context.Context, linkID string, fn func(*Click) error) error {
	rows, err := s.query(ctx, `SELECT id, link_id, ts, ip, ip_version, country, region, city, referrer, referrer_host, user_agent, device, os, os_version, browser, browser_version, is_bot, lang, utm_source, utm_medium, utm_campaign, utm_term, utm_content, qs FROM clicks WHERE link_id = ? ORDER BY ts ASC`, linkID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		c, err := scanClick(rows)
		if err != nil {
			return err
		}
		if err := fn(c); err != nil {
			return err
		}
	}
	return rows.Err()
}

func scanClick(row interface{ Scan(...any) error }) (*Click, error) {
	var c Click
	var tsMS int64
	var isBot int
	if err := row.Scan(&c.ID, &c.LinkID, &tsMS, &c.IP, &c.IPVersion, &c.Country, &c.Region, &c.City,
		&c.Referrer, &c.ReferrerHost, &c.UserAgent, &c.Device, &c.OS, &c.OSVersion, &c.Browser, &c.BrowserVersion,
		&isBot, &c.Lang, &c.UTMSource, &c.UTMMedium, &c.UTMCampaign, &c.UTMTerm, &c.UTMContent, &c.QS); err != nil {
		return nil, err
	}
	c.TS = fromMillis(tsMS)
	c.IsBot = isBot != 0
	return &c, nil
}

// TimeSeriesPoint is one bucket of the clicks-over-time chart.
type TimeSeriesPoint struct {
	Bucket  string // formatted per bucket size by caller
	Clicks  int64
	Bots    int64
	Uniques int64
}

// ClicksSeriesRaw buckets raw clicks by UTC day (bucket=day/week/month->day
// then re-bucketed in Go) or by hour, between from/to.
// Day and coarser granularities bucket by UTC day; hourly is for short ranges.
func (s *Store) ClicksSeriesRaw(ctx context.Context, linkID string, from, to time.Time, hourly bool) ([]TimeSeriesPoint, error) {
	var where []string
	var args []any
	if linkID != "" {
		where = append(where, "link_id = ?")
		args = append(args, linkID)
	}
	where = append(where, "ts >= ?", "ts < ?")
	args = append(args, toMillis(from), toMillis(to))
	return s.clicksSeriesRaw(ctx, strings.Join(where, " AND "), args, hourly)
}

// The ForUser variants below scope a query to one user's own links via a
// subquery on links.user_id — used for a non-admin's "my stats" dashboard so
// it never returns other users' click data.
// Kept as separate functions rather than adding a parameter to the
// link-scoped versions above, to avoid disturbing their existing call sites.

func (s *Store) ClicksSeriesRawForUser(ctx context.Context, userID string, from, to time.Time, hourly bool) ([]TimeSeriesPoint, error) {
	where := "ts >= ? AND ts < ? AND link_id IN (SELECT id FROM links WHERE user_id = ?)"
	return s.clicksSeriesRaw(ctx, where, []any{toMillis(from), toMillis(to), userID}, hourly)
}

// clicksSeriesRaw is the shared implementation both public variants route
// through: buckets clicks by UTC day/hour, counting clicks, bots, and
// per-bucket unique IPs (used for the dashboard "uniques" line, an
// approximation under IP_MODE=anonymize).
//
// Bucketing and aggregation both happen in SQL (dialect-specific date
// truncation) rather than by scanning every raw row into Go, so a link with
// millions of clicks over a wide date range doesn't have to load its entire
// click history into application memory just to draw a chart.
func (s *Store) clicksSeriesRaw(ctx context.Context, where string, args []any, hourly bool) ([]TimeSeriesPoint, error) {
	var bucketExpr string
	switch s.Driver {
	case "postgres":
		if hourly {
			bucketExpr = `to_char(to_timestamp(ts / 1000.0) AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24')`
		} else {
			bucketExpr = `to_char(to_timestamp(ts / 1000.0) AT TIME ZONE 'UTC', 'YYYY-MM-DD')`
		}
	default: // sqlite
		if hourly {
			bucketExpr = `strftime('%Y-%m-%dT%H', ts / 1000, 'unixepoch')`
		} else {
			bucketExpr = `strftime('%Y-%m-%d', ts / 1000, 'unixepoch')`
		}
	}

	q := `SELECT ` + bucketExpr + ` AS bucket,
			SUM(CASE WHEN is_bot = 0 THEN 1 ELSE 0 END) AS clicks,
			SUM(CASE WHEN is_bot != 0 THEN 1 ELSE 0 END) AS bots,
			COUNT(DISTINCT CASE WHEN is_bot = 0 THEN ip END) AS uniques
		FROM clicks WHERE ` + where + `
		GROUP BY bucket ORDER BY bucket`

	rows, err := s.query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TimeSeriesPoint
	for rows.Next() {
		var p TimeSeriesPoint
		if err := rows.Scan(&p.Bucket, &p.Clicks, &p.Bots, &p.Uniques); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) TotalClicksForUser(ctx context.Context, userID string, from, to time.Time, includeBots bool) (int64, error) {
	q := `SELECT count(*) FROM clicks WHERE ts >= ? AND ts < ? AND link_id IN (SELECT id FROM links WHERE user_id = ?)`
	if !includeBots {
		q += ` AND is_bot = 0`
	}
	var n int64
	err := s.queryRow(ctx, q, toMillis(from), toMillis(to), userID).Scan(&n)
	return n, err
}

func (s *Store) UniqueVisitorsForUser(ctx context.Context, userID string, from, to time.Time) (int64, error) {
	var n int64
	err := s.queryRow(ctx, `SELECT count(DISTINCT ip) FROM clicks WHERE ts >= ? AND ts < ? AND is_bot = 0 AND link_id IN (SELECT id FROM links WHERE user_id = ?)`,
		toMillis(from), toMillis(to), userID).Scan(&n)
	return n, err
}

func (s *Store) ClicksBreakdownForUser(ctx context.Context, userID, dim string, from, to time.Time, topN int) ([]BreakdownRow, error) {
	col := map[string]string{
		"country": "country", "device": "device", "os": "os", "browser": "browser", "referrer_host": "referrer_host",
	}[dim]
	if col == "" {
		return nil, fmt.Errorf("unknown breakdown dimension %q", dim)
	}
	q := `SELECT ` + col + `, count(*) as c FROM clicks WHERE ts >= ? AND ts < ? AND is_bot = 0 AND link_id IN (SELECT id FROM links WHERE user_id = ?) GROUP BY ` + col + ` ORDER BY c DESC LIMIT ?`
	rows, err := s.query(ctx, q, toMillis(from), toMillis(to), userID, topN)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BreakdownRow
	for rows.Next() {
		var r BreakdownRow
		if err := rows.Scan(&r.Key, &r.Clicks); err != nil {
			return nil, err
		}
		if r.Key == "" {
			r.Key = "(direct)"
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// BreakdownRow is one row of a dimension breakdown (country/device/os/browser/referrer_host).
type BreakdownRow struct {
	Key    string
	Clicks int64
}

func (s *Store) ClicksBreakdown(ctx context.Context, linkID, dim string, from, to time.Time, topN int) ([]BreakdownRow, error) {
	col := map[string]string{
		"country": "country", "device": "device", "os": "os", "browser": "browser", "referrer_host": "referrer_host",
	}[dim]
	if col == "" {
		return nil, fmt.Errorf("unknown breakdown dimension %q", dim)
	}
	var where []string
	var args []any
	if linkID != "" {
		where = append(where, "link_id = ?")
		args = append(args, linkID)
	}
	where = append(where, "ts >= ?", "ts < ?", "is_bot = 0")
	args = append(args, toMillis(from), toMillis(to))
	sqlStr := `SELECT ` + col + `, count(*) as c FROM clicks WHERE ` + strings.Join(where, " AND ") + ` GROUP BY ` + col + ` ORDER BY c DESC LIMIT ?`
	args = append(args, topN)
	rows, err := s.query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BreakdownRow
	for rows.Next() {
		var r BreakdownRow
		if err := rows.Scan(&r.Key, &r.Clicks); err != nil {
			return nil, err
		}
		if r.Key == "" {
			r.Key = "(direct)"
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) UniqueVisitors(ctx context.Context, linkID string, from, to time.Time) (int64, error) {
	var where []string
	var args []any
	if linkID != "" {
		where = append(where, "link_id = ?")
		args = append(args, linkID)
	}
	where = append(where, "ts >= ?", "ts < ?", "is_bot = 0")
	args = append(args, toMillis(from), toMillis(to))
	var n int64
	err := s.queryRow(ctx, `SELECT count(DISTINCT ip) FROM clicks WHERE `+strings.Join(where, " AND "), args...).Scan(&n)
	return n, err
}

func (s *Store) TotalClicks(ctx context.Context, linkID string, from, to time.Time, includeBots bool) (int64, error) {
	var where []string
	var args []any
	if linkID != "" {
		where = append(where, "link_id = ?")
		args = append(args, linkID)
	}
	where = append(where, "ts >= ?", "ts < ?")
	args = append(args, toMillis(from), toMillis(to))
	if !includeBots {
		where = append(where, "is_bot = 0")
	}
	var n int64
	err := s.queryRow(ctx, `SELECT count(*) FROM clicks WHERE `+strings.Join(where, " AND "), args...).Scan(&n)
	return n, err
}

// DeleteClicksOlderThan removes raw clicks before cutoff, in chunks, to
// avoid a single long-running lock.
func (s *Store) DeleteClicksOlderThan(ctx context.Context, cutoff time.Time, chunk int) (int64, error) {
	var total int64
	for {
		res, err := s.execWrite(ctx, `DELETE FROM clicks WHERE id IN (SELECT id FROM clicks WHERE ts < ? LIMIT ?)`, toMillis(cutoff), chunk)
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += n
		if n < int64(chunk) {
			break
		}
	}
	return total, nil
}

func (s *Store) RecomputeClickCount(ctx context.Context, linkID string) error {
	var total int64
	var lastTS sql.NullInt64
	if err := s.queryRow(ctx, `SELECT count(*), max(ts) FROM clicks WHERE link_id = ? AND is_bot = 0`, linkID).Scan(&total, &lastTS); err != nil {
		return err
	}
	_, err := s.execWrite(ctx, `UPDATE links SET click_count = ?, last_click_at = COALESCE(?, last_click_at) WHERE id = ?`, total, lastTS, linkID)
	return err
}
