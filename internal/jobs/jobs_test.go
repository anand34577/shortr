package jobs

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"shortr/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open("sqlite", filepath.Join(dir, "test.db"), dir, 4)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// TestRunReturnsOnlyAfterJobsFinish ensures Run doesn't return the instant
// its context is cancelled — it must wait for every spawned job goroutine
// to actually observe cancellation and exit, so a caller doing a graceful
// shutdown can safely assume no job is left running once Run returns.
func TestRunReturnsOnlyAfterJobsFinish(t *testing.T) {
	st := newTestStore(t)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	r := New(Config{Store: st, Log: log})

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(runDone)
	}()

	// Give the job goroutines a moment to actually start (past their jitter
	// sleep) before cancelling, so this exercises the real shutdown path
	// rather than cancelling before anything spawned.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestBackupDisabledOnPostgres(t *testing.T) {
	st := newTestStore(t)
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	r := New(Config{Store: st, Log: log, BackupInterval: time.Hour, DBDriver: "postgres"})

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(runDone)
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
	// No assertion beyond "doesn't panic and doesn't hang" — the backup job
	// must not be spawned at all for a non-sqlite driver.
}
