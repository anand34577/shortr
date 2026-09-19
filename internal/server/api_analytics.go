package server

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"shortr/internal/store"
)

func parseRange(r *http.Request) (from, to time.Time, err error) {
	q := r.URL.Query()
	to = time.Now()
	from = to.Add(-30 * 24 * time.Hour)
	if v := q.Get("to"); v != "" {
		to, err = parseFlexTime(v)
		if err != nil {
			return
		}
	}
	if v := q.Get("from"); v != "" {
		from, err = parseFlexTime(v)
		if err != nil {
			return
		}
	}
	if from.After(to) {
		err = fmt.Errorf("from must be before to")
		return
	}
	if to.Sub(from) > 2*365*24*time.Hour {
		err = fmt.Errorf("date range must be at most 2 years")
		return
	}
	return
}

func parseFlexTime(v string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", v)
}

type statsResp struct {
	From      time.Time             `json:"from"`
	To        time.Time             `json:"to"`
	Total     int64                 `json:"total_clicks"`
	Uniques   int64                 `json:"unique_visitors"`
	Series    []seriesPointDTO      `json:"series"`
	Breakdown map[string][]bdRowDTO `json:"breakdown"`
}
type seriesPointDTO struct {
	Bucket string `json:"bucket"`
	Clicks int64  `json:"clicks"`
	Bots   int64  `json:"bots"`
}
type bdRowDTO struct {
	Key    string `json:"key"`
	Clicks int64  `json:"clicks"`
}

func (s *Server) handleLinkStats(w http.ResponseWriter, r *http.Request, u *store.User, id string) {
	l := s.getOwnedLink(w, r, u, id)
	if l == nil {
		return
	}
	from, to, err := parseRange(r)
	if err != nil {
		respondError(w, r, NewAPIError(http.StatusBadRequest, "BAD_REQUEST", err.Error()))
		return
	}
	// a single link's own stats are unambiguous: no cross-user leak risk
	// regardless of who owns it, since getOwnedLink already checked access.
	s.writeStats(w, r, statsScope{linkID: l.ID}, from, to)
}

func (s *Server) handleGlobalStats(w http.ResponseWriter, r *http.Request, u *store.User) {
	from, to, err := parseRange(r)
	if err != nil {
		respondError(w, r, NewAPIError(http.StatusBadRequest, "BAD_REQUEST", err.Error()))
		return
	}
	scope := statsScope{userID: u.ID}
	if u.IsAdmin() && r.URL.Query().Get("scope") == "all" {
		scope = statsScope{} // unscoped: every link, every user
	}
	s.writeStats(w, r, scope, from, to)
}

// statsScope pins a stats query to exactly one of: a single link (owner
// already verified by the caller), one user's own links, or nothing (admin
// "all" view). Never leave userID and linkID both empty unless the caller is
// an admin explicitly asking for the platform-wide view.
type statsScope struct {
	linkID string
	userID string
}

func (s *Server) writeStats(w http.ResponseWriter, r *http.Request, scope statsScope, from, to time.Time) {
	hourly := to.Sub(from) <= 3*24*time.Hour

	var points []store.TimeSeriesPoint
	var total, uniques int64
	var err error
	breakdown := map[string][]bdRowDTO{}

	switch {
	case scope.linkID != "":
		points, err = s.store.ClicksSeriesRaw(r.Context(), scope.linkID, from, to, hourly)
		if err == nil {
			total, err = s.store.TotalClicks(r.Context(), scope.linkID, from, to, false)
		}
		if err == nil {
			uniques, err = s.store.UniqueVisitors(r.Context(), scope.linkID, from, to)
		}
		for _, dim := range []string{"country", "device", "os", "browser", "referrer_host"} {
			if rows, e := s.store.ClicksBreakdown(r.Context(), scope.linkID, dim, from, to, 10); e == nil {
				breakdown[dim] = toBDRows(rows)
			}
		}
	case scope.userID != "":
		points, err = s.store.ClicksSeriesRawForUser(r.Context(), scope.userID, from, to, hourly)
		if err == nil {
			total, err = s.store.TotalClicksForUser(r.Context(), scope.userID, from, to, false)
		}
		if err == nil {
			uniques, err = s.store.UniqueVisitorsForUser(r.Context(), scope.userID, from, to)
		}
		for _, dim := range []string{"country", "device", "os", "browser", "referrer_host"} {
			if rows, e := s.store.ClicksBreakdownForUser(r.Context(), scope.userID, dim, from, to, 10); e == nil {
				breakdown[dim] = toBDRows(rows)
			}
		}
	default:
		points, err = s.store.ClicksSeriesRaw(r.Context(), "", from, to, hourly)
		if err == nil {
			total, err = s.store.TotalClicks(r.Context(), "", from, to, false)
		}
		if err == nil {
			uniques, err = s.store.UniqueVisitors(r.Context(), "", from, to)
		}
		for _, dim := range []string{"country", "device", "os", "browser", "referrer_host"} {
			if rows, e := s.store.ClicksBreakdown(r.Context(), "", dim, from, to, 10); e == nil {
				breakdown[dim] = toBDRows(rows)
			}
		}
	}
	if err != nil {
		respondError(w, r, err)
		return
	}

	series := make([]seriesPointDTO, 0, len(points))
	for _, p := range points {
		series = append(series, seriesPointDTO{Bucket: p.Bucket, Clicks: p.Clicks, Bots: p.Bots})
	}
	respondJSON(w, http.StatusOK, statsResp{From: from, To: to, Total: total, Uniques: uniques, Series: series, Breakdown: breakdown})
}

func toBDRows(rows []store.BreakdownRow) []bdRowDTO {
	out := make([]bdRowDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, bdRowDTO{Key: row.Key, Clicks: row.Clicks})
	}
	return out
}

type clickDTO struct {
	Time     time.Time `json:"time"`
	Country  string    `json:"country"`
	Device   string    `json:"device"`
	OS       string    `json:"os"`
	Browser  string    `json:"browser"`
	Referrer string    `json:"referrer_host"`
	IsBot    bool      `json:"is_bot"`
}

func (s *Server) handleListClicks(w http.ResponseWriter, r *http.Request, u *store.User, id string) {
	l := s.getOwnedLink(w, r, u, id)
	if l == nil {
		return
	}
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, _ = strconv.Atoi(v)
	}
	clicks, next, err := s.store.ListClicks(r.Context(), store.ClickFilter{LinkID: l.ID, Cursor: r.URL.Query().Get("cursor"), Limit: limit})
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]clickDTO, 0, len(clicks))
	for _, c := range clicks {
		out = append(out, clickDTO{Time: c.TS, Country: c.Country, Device: c.Device, OS: c.OS, Browser: c.Browser, Referrer: c.ReferrerHost, IsBot: c.IsBot})
	}
	respondList(w, out, next)
}

func (s *Server) handleExportClicks(w http.ResponseWriter, r *http.Request, u *store.User, id string) {
	l := s.getOwnedLink(w, r, u, id)
	if l == nil {
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+l.Code+`-clicks.csv"`)
	cw := csv.NewWriter(w)
	cw.Write([]string{"time", "country", "region", "city", "device", "os", "browser", "referrer_host", "is_bot", "utm_source", "utm_medium", "utm_campaign"}) //nolint:errcheck

	err := s.store.StreamClicks(r.Context(), l.ID, func(c *store.Click) error {
		row := []string{
			c.TS.UTC().Format(time.RFC3339), csvSafe(c.Country), csvSafe(c.Region), csvSafe(c.City),
			csvSafe(c.Device), csvSafe(c.OS), csvSafe(c.Browser), csvSafe(c.ReferrerHost),
			strconv.FormatBool(c.IsBot), csvSafe(c.UTMSource), csvSafe(c.UTMMedium), csvSafe(c.UTMCampaign),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
		cw.Flush()
		return cw.Error()
	})
	if err != nil {
		s.log.Warn("click export interrupted", "error", err)
	}
}

// csvSafe prefixes a leading =+-@ with a single quote to defeat formula
// injection when the CSV is opened in a spreadsheet app (PLAN.md §10.4).
func csvSafe(s string) string {
	if len(s) > 0 {
		switch s[0] {
		case '=', '+', '-', '@':
			return "'" + s
		}
	}
	return s
}
