package click

import (
	"context"
	"net"
	"time"
)

const (
	backfillBatch  = 200
	backfillWindow = 7 * 24 * time.Hour
)

// BackfillGeo fills in country/region/city for recent clicks that were
// recorded while the IP location service was down or not yet configured. It
// only works when IPs are stored in full (anonymised or hashed IPs can't be
// located), does a bounded batch per call, and stops as soon as the service
// is unavailable, so it can run on a timer without ever hammering it.
func (w *Writer) BackfillGeo(ctx context.Context) error {
	if w.cfg.IPMode != "full" || !w.ipLoc.available() {
		return nil
	}
	rows, err := w.store.ClicksMissingGeo(ctx, time.Now().Add(-backfillWindow), w.backfillCursor, backfillBatch)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		w.backfillCursor = 0 // start over from the newest next time
		return nil
	}
	for _, c := range rows {
		if ctx.Err() != nil || !w.ipLoc.available() {
			return ctx.Err()
		}
		w.backfillCursor = c.ID // rows the service can't place are skipped next run, not retried forever
		ip := net.ParseIP(c.IP)
		if ip == nil {
			continue
		}
		g := w.geo.Lookup(ip)
		if g.Country == "" {
			g = w.ipLoc.Lookup(ip)
		}
		if g.Country == "" {
			continue
		}
		if err := w.store.SetClickGeo(ctx, c.ID, g.Country, g.Region, g.City); err != nil {
			return err
		}
	}
	return nil
}
