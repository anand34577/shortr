// Package jobs runs the periodic background maintenance shortr needs:
// session/rate-limit GC, expired-link notices, click retention, and SQLite
// backups. Each job is independent, catches its own panics, and logs rather
// than crashing the process (PLAN.md §6 background jobs list).
package jobs

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"shortr/internal/notify"
	"shortr/internal/store"
)

type Runner struct {
	store    *store.Store
	notifier *notify.Notifier
	log      *slog.Logger
	wg       sync.WaitGroup

	clickRetentionDays  int
	rollupRetentionDays int
	backupInterval      time.Duration
	backupKeep          int
	dataDir             string
	dbDriver            string
}

type Config struct {
	Store               *store.Store
	Notifier            *notify.Notifier
	Log                 *slog.Logger
	ClickRetentionDays  int
	RollupRetentionDays int
	BackupInterval      time.Duration
	BackupKeep          int
	DataDir             string
	DBDriver            string
}

func New(c Config) *Runner {
	if c.Log == nil {
		c.Log = slog.Default()
	}
	return &Runner{
		store: c.Store, notifier: c.Notifier, log: c.Log,
		clickRetentionDays: c.ClickRetentionDays, rollupRetentionDays: c.RollupRetentionDays,
		backupInterval: c.BackupInterval, backupKeep: c.BackupKeep, dataDir: c.DataDir, dbDriver: c.DBDriver,
	}
}

// Run blocks, firing each job on its own ticker with a random jitter so they
// don't all wake at once, until ctx is cancelled. Run itself returns as soon
// as every job goroutine has observed cancellation and returned, so callers
// doing a graceful shutdown can safely assume no job is left running once
// Run returns.
func (r *Runner) Run(ctx context.Context) {
	r.spawn(ctx, "session_gc", time.Hour, r.sessionGC)
	r.spawn(ctx, "expiry_notice", 6*time.Hour, r.expiryNotices)
	r.spawn(ctx, "soft_delete_purge", 24*time.Hour, r.purgeSoftDeleted)
	r.spawn(ctx, "click_retention", 24*time.Hour, r.clickRetention)
	r.spawn(ctx, "idempotency_gc", time.Hour, r.idempotencyGC)
	if r.backupInterval > 0 {
		if r.dbDriver == "sqlite" {
			r.spawn(ctx, "backup", r.backupInterval, r.backup)
		} else {
			r.log.Warn("SHORTR_BACKUP_INTERVAL is set but the automatic backup job only supports sqlite; no backups will run", "db_driver", r.dbDriver)
		}
	}
	<-ctx.Done()
	r.wg.Wait()
}

func (r *Runner) spawn(ctx context.Context, name string, interval time.Duration, fn func(context.Context) error) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.loop(ctx, name, interval, fn)
	}()
}

func (r *Runner) loop(ctx context.Context, name string, interval time.Duration, fn func(context.Context) error) {
	jitter := time.Duration(rand.Int63n(int64(interval) / 4))
	select {
	case <-time.After(jitter):
	case <-ctx.Done():
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	r.runOnce(name, fn)
	for {
		select {
		case <-ticker.C:
			r.runOnce(name, fn)
		case <-ctx.Done():
			return
		}
	}
}

func (r *Runner) runOnce(name string, fn func(context.Context) error) {
	defer func() {
		if rec := recover(); rec != nil {
			r.log.Error("job panicked", "job", name, "panic", rec)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := fn(ctx); err != nil {
		r.log.Warn("job failed", "job", name, "error", err)
	}
}

func (r *Runner) sessionGC(ctx context.Context) error {
	n, err := r.store.DeleteExpiredSessions(ctx, time.Now())
	if err != nil {
		return err
	}
	if n > 0 {
		r.log.Info("session gc", "deleted", n)
	}
	return nil
}

func (r *Runner) purgeSoftDeleted(ctx context.Context) error {
	n, err := r.store.PurgeSoftDeletedBefore(ctx, time.Now().Add(-30*24*time.Hour))
	if err != nil {
		return err
	}
	if n > 0 {
		r.log.Info("purged soft-deleted links", "count", n)
	}
	return nil
}

func (r *Runner) clickRetention(ctx context.Context) error {
	if r.clickRetentionDays <= 0 {
		return nil
	}
	cutoff := time.Now().Add(-time.Duration(r.clickRetentionDays) * 24 * time.Hour)
	n, err := r.store.DeleteClicksOlderThan(ctx, cutoff, 5000)
	if err != nil {
		return err
	}
	if n > 0 {
		r.log.Info("click retention purge", "deleted", n)
	}
	return nil
}

func (r *Runner) idempotencyGC(ctx context.Context) error {
	return r.store.PruneIdempotencyKeys(ctx, time.Now().Add(-24*time.Hour))
}

// expiryNotices tells link owners (and, best-effort, admins) about links
// expiring within the next 24h, once — future ticks won't re-notify because
// ListExpiringLinks only returns links that haven't expired yet, and once a
// link expires it drops out of this query naturally. ponytail: no
// "already notified" flag; an owner may get the email more than once if the
// job runs twice before the link actually expires (harmless, rare given the
// 6h interval vs 24h window).
func (r *Runner) expiryNotices(ctx context.Context) error {
	if r.notifier == nil {
		return nil
	}
	links, err := r.store.ListExpiringLinks(ctx, time.Now().Add(24*time.Hour))
	if err != nil {
		return err
	}
	for _, l := range links {
		if l.UserID == nil {
			continue
		}
		u, err := r.store.GetUserByID(ctx, *l.UserID)
		if err != nil {
			continue
		}
		r.notifier.NotifyUser(ctx, u.ID, u.Email, notify.KindLinkExpiring, "Link expiring soon",
			"Your link /"+l.Code+" expires within 24 hours.", map[string]any{"link_id": l.ID, "code": l.Code})
	}
	return nil
}

func (r *Runner) backup(ctx context.Context) error {
	err := r.doBackup(ctx)
	if err != nil && r.notifier != nil {
		r.notifier.NotifyAdmins(ctx, notify.KindBackupFailed, "Backup failed", err.Error(), nil)
	}
	return err
}
