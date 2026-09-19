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
	Bucket string // formatted per bucket size by caller
	Clicks int64
	Bots   int64
}

// ClicksSeriesRaw buckets raw clicks by UTC day (bucket=day/week/month->day
// then re-bucketed in Go) or by hour, between from/to. See PLAN.md §10.5:
// day+ granularity buckets by UTC day; hour granularity used for short ranges.
func (s *Store) ClicksSeriesRaw(ctx context.Context, linkID string, from, to time.Time, hourly bool) ([]TimeSeriesPoint, error) {
	var where []string
	var args []any
	if linkID != "" {
		where = append(where, "link_id = ?")
		args = append(args, linkID)
	}
	where = append(where, "ts >= ?", "ts < ?")
	args = append(args, toMillis(from), toMillis(to))

	// bucket key computed in Go from ts (portable across sqlite/postgres)
	rows, err := s.query(ctx, `SELECT ts, is_bot FROM clicks WHERE `+strings.Join(where, " AND "), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	buckets := map[string]*TimeSeriesPoint{}
	var order []string
	for rows.Next() {
		var tsMS int64
		var isBot int
		if err := rows.Scan(&tsMS, &isBot); err != nil {
			return nil, err
		}
		t := fromMillis(tsMS).UTC()
		key := t.Format("2006-01-02")
		if hourly {
			key = t.Format("2006-01-02T15")
		}
		p, ok := buckets[key]
		if !ok {
			p = &TimeSeriesPoint{Bucket: key}
			buckets[key] = p
			order = append(order, key)
		}
		if isBot != 0 {
			p.Bots++
		} else {
			p.Clicks++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sortStrings(order)
	out := make([]TimeSeriesPoint, 0, len(order))
	for _, k := range order {
		out = append(out, *buckets[k])
	}
	return out, nil
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
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
// avoid a single long-running lock (PLAN.md §10.4).
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
