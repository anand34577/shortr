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
	if !validTZ(q.Get("tz")) {
		err = fmt.Errorf("tz must be a valid IANA time zone")
		return
	}
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

// Response shapes below mirror web/src/lib/types.ts exactly (LinkStats and
// StatsOverview) — the two stats endpoints have genuinely different shapes
// (per-link breakdown-heavy vs cross-link totals-and-leaderboard), so they
// get separate DTOs rather than one shared "stats" envelope.

type seriesPointDTO struct {
	Bucket  string `json:"bucket"`
	Clicks  int64  `json:"clicks"`
	Uniques int64  `json:"uniques"`
	Bots    int64  `json:"bots"`
}

type bdRowDTO struct {
	Key    string  `json:"key"`
	Clicks int64   `json:"clicks"`
	Pct    float64 `json:"pct"`
}

func toSeriesDTO(points []store.TimeSeriesPoint) []seriesPointDTO {
	out := make([]seriesPointDTO, 0, len(points))
	for _, p := range points {
		out = append(out, seriesPointDTO{Bucket: p.Bucket, Clicks: p.Clicks, Uniques: p.Uniques, Bots: p.Bots})
	}
	return out
}

func toBDRows(rows []store.BreakdownRow) []bdRowDTO {
	var total int64
	for _, row := range rows {
		total += row.Clicks
	}
	out := make([]bdRowDTO, 0, len(rows))
	for _, row := range rows {
		pct := 0.0
		if total > 0 {
			pct = float64(row.Clicks) / float64(total) * 100
		}
		out = append(out, bdRowDTO{Key: row.Key, Clicks: row.Clicks, Pct: pct})
	}
	return out
}

type linkStatsResp struct {
	Series     []seriesPointDTO `json:"series"`
	Totals     totalsDTO        `json:"totals"`
	ByCountry  []bdRowDTO       `json:"byCountry"`
	ByDevice   []bdRowDTO       `json:"byDevice"`
	ByOS       []bdRowDTO       `json:"byOS"`
	ByBrowser  []bdRowDTO       `json:"byBrowser"`
	ByReferrer []bdRowDTO       `json:"byReferrer"`
}

type totalsDTO struct {
	Clicks  int64 `json:"clicks"`
	Uniques int64 `json:"uniques"`
	Bots    int64 `json:"bots,omitempty"`
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
	hourly := to.Sub(from) <= 3*24*time.Hour
	ctx := r.Context()

	points, err := s.store.ClicksSeriesRaw(ctx, l.ID, from, to, hourly)
	if err != nil {
		respondError(w, r, err)
		return
	}
	totalClicks, err := s.store.TotalClicks(ctx, l.ID, from, to, false)
	if err != nil {
		respondError(w, r, err)
		return
	}
	totalBots, err := s.store.TotalClicks(ctx, l.ID, from, to, true)
	if err != nil {
		respondError(w, r, err)
		return
	}
	totalBots -= totalClicks // TotalClicks(includeBots=true) counts both; subtract the non-bot count
	uniques, err := s.store.UniqueVisitors(ctx, l.ID, from, to)
	if err != nil {
		respondError(w, r, err)
		return
	}

	byCountry, _ := s.store.ClicksBreakdown(ctx, l.ID, "country", from, to, 10)
	byDevice, _ := s.store.ClicksBreakdown(ctx, l.ID, "device", from, to, 10)
	byOS, _ := s.store.ClicksBreakdown(ctx, l.ID, "os", from, to, 10)
	byBrowser, _ := s.store.ClicksBreakdown(ctx, l.ID, "browser", from, to, 10)
	byReferrer, _ := s.store.ClicksBreakdown(ctx, l.ID, "referrer_host", from, to, 10)

	respondJSON(w, http.StatusOK, linkStatsResp{
		Series: toSeriesDTO(points), Totals: totalsDTO{Clicks: totalClicks, Uniques: uniques, Bots: totalBots},
		ByCountry: toBDRows(byCountry), ByDevice: toBDRows(byDevice), ByOS: toBDRows(byOS),
		ByBrowser: toBDRows(byBrowser), ByReferrer: toBDRows(byReferrer),
	})
}

type overviewTotalsDTO struct {
	Clicks      int64 `json:"clicks"`
	Uniques     int64 `json:"uniques"`
	ActiveLinks int64 `json:"activeLinks"`
}

type overviewResp struct {
	Totals      overviewTotalsDTO `json:"totals"`
	Series      []seriesPointDTO  `json:"series"`
	TopLinks    []linkDTO         `json:"topLinks"`
	TopReferrer *string           `json:"topReferrer"`
	DeltaPct    *float64          `json:"deltaPct"`
}

func (s *Server) handleGlobalStats(w http.ResponseWriter, r *http.Request, u *store.User) {
	from, to, err := parseRange(r)
	if err != nil {
		respondError(w, r, NewAPIError(http.StatusBadRequest, "BAD_REQUEST", err.Error()))
		return
	}
	ctx := r.Context()
	scopeUserID := u.ID
	all := u.IsAdmin() && r.URL.Query().Get("scope") == "all"
	if all {
		scopeUserID = ""
	}

	var points []store.TimeSeriesPoint
	var totalClicks, uniques int64
	var referrerRows []store.BreakdownRow
	if scopeUserID != "" {
		points, err = s.store.ClicksSeriesRawForUser(ctx, scopeUserID, from, to, to.Sub(from) <= 3*24*time.Hour)
		if err == nil {
			totalClicks, err = s.store.TotalClicksForUser(ctx, scopeUserID, from, to, false)
		}
		if err == nil {
			uniques, err = s.store.UniqueVisitorsForUser(ctx, scopeUserID, from, to)
		}
		if err == nil {
			referrerRows, _ = s.store.ClicksBreakdownForUser(ctx, scopeUserID, "referrer_host", from, to, 1)
		}
	} else {
		points, err = s.store.ClicksSeriesRaw(ctx, "", from, to, to.Sub(from) <= 3*24*time.Hour)
		if err == nil {
			totalClicks, err = s.store.TotalClicks(ctx, "", from, to, false)
		}
		if err == nil {
			uniques, err = s.store.UniqueVisitors(ctx, "", from, to)
		}
		if err == nil {
			referrerRows, _ = s.store.ClicksBreakdown(ctx, "", "referrer_host", from, to, 1)
		}
	}
	if err != nil {
		respondError(w, r, err)
		return
	}

	activeLinks, err := s.store.CountActiveLinks(ctx, scopeUserID)
	if err != nil {
		respondError(w, r, err)
		return
	}

	links, _, err := s.store.ListLinks(ctx, store.LinkFilter{UserID: scopeUserID, Sort: "clicks", Order: "desc", Limit: 5})
	if err != nil {
		respondError(w, r, err)
		return
	}
	topLinks := make([]linkDTO, 0, len(links))
	for _, l := range links {
		topLinks = append(topLinks, s.toLinkDTO(l))
	}

	var topReferrer *string
	if len(referrerRows) > 0 && referrerRows[0].Key != "(direct)" {
		topReferrer = &referrerRows[0].Key
	}

	// previous period of equal length, for the "vs last period" delta
	span := to.Sub(from)
	prevFrom, prevTo := from.Add(-span), from
	var prevTotal int64
	if scopeUserID != "" {
		prevTotal, _ = s.store.TotalClicksForUser(ctx, scopeUserID, prevFrom, prevTo, false)
	} else {
		prevTotal, _ = s.store.TotalClicks(ctx, "", prevFrom, prevTo, false)
	}
	var deltaPct *float64
	if prevTotal > 0 {
		d := (float64(totalClicks) - float64(prevTotal)) / float64(prevTotal) * 100
		deltaPct = &d
	}

	respondJSON(w, http.StatusOK, overviewResp{
		Totals: overviewTotalsDTO{Clicks: totalClicks, Uniques: uniques, ActiveLinks: int64(activeLinks)},
		Series: toSeriesDTO(points), TopLinks: topLinks, TopReferrer: topReferrer, DeltaPct: deltaPct,
	})
}

type recentActivityDTO struct {
	ID           string    `json:"id"`
	LinkID       string    `json:"linkId"`
	Code         string    `json:"code"`
	ShortURL     string    `json:"shortUrl"`
	Country      string    `json:"country"`
	Device       string    `json:"device"`
	TS           time.Time `json:"ts"`
	ReferrerHost string    `json:"referrerHost"`
}

func (s *Server) handleRecentActivity(w http.ResponseWriter, r *http.Request, u *store.User) {
	limit := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		limit, _ = strconv.Atoi(v)
	}
	scopeUserID := u.ID
	if u.IsAdmin() && r.URL.Query().Get("scope") == "all" {
		scopeUserID = ""
	}
	rows, err := s.store.ListRecentActivity(r.Context(), scopeUserID, limit)
	if err != nil {
		respondError(w, r, err)
		return
	}
	out := make([]recentActivityDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, recentActivityDTO{
			ID: fmt.Sprint(row.ClickID), LinkID: row.LinkID, Code: row.Code, ShortURL: s.links.ShortURL(row.Code),
			Country: row.Country, Device: row.Device, TS: row.TS, ReferrerHost: row.ReferrerHost,
		})
	}
	respondList(w, out, "")
}

type clickDTO struct {
	ID           int64     `json:"id"`
	LinkID       string    `json:"linkId"`
	TS           time.Time `json:"ts"`
	IP           string    `json:"ip"`
	Country      string    `json:"country"`
	Region       string    `json:"region"`
	City         string    `json:"city"`
	Referrer     string    `json:"referrer"`
	ReferrerHost string    `json:"referrerHost"`
	Device       string    `json:"device"`
	OS           string    `json:"os"`
	Browser      string    `json:"browser"`
	IsBot        bool      `json:"isBot"`
	Lang         string    `json:"lang"`
	UTM          utmDTO    `json:"utm"`
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
		out = append(out, clickDTO{
			ID: c.ID, LinkID: c.LinkID, TS: c.TS, IP: c.IP, Country: c.Country, Region: c.Region, City: c.City,
			Referrer: c.Referrer, ReferrerHost: c.ReferrerHost, Device: c.Device, OS: c.OS, Browser: c.Browser,
			IsBot: c.IsBot, Lang: c.Lang,
			UTM: utmDTO{c.UTMSource, c.UTMMedium, c.UTMCampaign, c.UTMTerm, c.UTMContent},
		})
	}
	respondList(w, out, next)
}

func (s *Server) handleExportClicks(w http.ResponseWriter, r *http.Request, u *store.User, id string) {
	l := s.getOwnedLink(w, r, u, id)
	if l == nil {
		return
	}
	if r.URL.Query().Get("format") == "json" {
		s.exportClicksJSON(w, r, l)
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
