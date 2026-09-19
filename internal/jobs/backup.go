package jobs

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// backup runs VACUUM INTO a timestamped file under <dataDir>/backups/ and
// keeps only the most recent `keep` snapshots (PLAN.md §22.6, §6
// SHORTR_BACKUP_KEEP).
func (r *Runner) doBackup(ctx context.Context) error {
	dir := filepath.Join(r.dataDir, "backups")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	name := filepath.Join(dir, "shortr-"+time.Now().UTC().Format("20060102T150405")+".db")
	if err := r.store.BackupSQLite(ctx, name); err != nil {
		return err
	}
	r.log.Info("backup written", "path", name)
	return pruneBackups(dir, r.backupKeep)
}

func pruneBackups(dir string, keep int) error {
	if keep <= 0 {
		keep = 7
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".db" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names) // timestamped names sort chronologically
	if len(names) <= keep {
		return nil
	}
	for _, n := range names[:len(names)-keep] {
		_ = os.Remove(filepath.Join(dir, n))
	}
	return nil
}
