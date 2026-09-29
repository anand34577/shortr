package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockSurvivesStaleFileButBlocksSecondOpen(t *testing.T) {
	dir := t.TempDir()
	// a lock file left behind by a crashed process must not block startup
	if err := os.WriteFile(filepath.Join(dir, "shortr.lock"), []byte("99999"), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := Open("sqlite", filepath.Join(dir, "a.db"), dir, 2)
	if err != nil {
		t.Fatalf("stale lock file blocked open: %v", err)
	}
	if _, err := Open("sqlite", filepath.Join(dir, "a.db"), dir, 2); err == nil {
		t.Fatal("second instance was allowed to open the same data dir")
	}
	st.Close()
	st2, err := Open("sqlite", filepath.Join(dir, "a.db"), dir, 2)
	if err != nil {
		t.Fatalf("reopen after close: %v", err)
	}
	st2.Close()
}
