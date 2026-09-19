// Package store is the only package that knows SQL. It supports SQLite
// (default, pure-Go modernc.org/sqlite, CGO-free) and PostgreSQL (pgx),
// through the same database/sql API and a small dialect-rewriting layer.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

//go:embed migrations/sqlite/*.sql
var sqliteMigrations embed.FS

//go:embed migrations/postgres/*.sql
var postgresMigrations embed.FS

type Store struct {
	Driver string // "sqlite" | "postgres"

	// SQLite: write is a single-connection pool that serialises all writes
	// (eliminates SQLITE_BUSY entirely); read is a multi-connection pool for
	// concurrent reads. Postgres: both point at the same pooled *sql.DB.
	write *sql.DB
	read  *sql.DB

	dataDir string
	lockFile *os.File
}

// Open connects, applies PRAGMAs/settings, and runs pending migrations.
func Open(driver, dsn, dataDir string, maxConns int) (*Store, error) {
	s := &Store{Driver: driver, dataDir: dataDir}

	switch driver {
	case "sqlite":
		if err := s.acquireLock(dataDir); err != nil {
			return nil, err
		}
		if dir := filepath.Dir(dsn); dir != "." {
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return nil, fmt.Errorf("creating db directory: %w", err)
			}
		}
		dsnFull := "file:" + dsn + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=cache_size(-20000)&_pragma=temp_store(MEMORY)"
		w, err := sql.Open("sqlite", dsnFull)
		if err != nil {
			return nil, fmt.Errorf("opening sqlite (write): %w", err)
		}
		w.SetMaxOpenConns(1)
		r, err := sql.Open("sqlite", dsnFull)
		if err != nil {
			return nil, fmt.Errorf("opening sqlite (read): %w", err)
		}
		if maxConns < 2 {
			maxConns = 8
		}
		r.SetMaxOpenConns(maxConns)
		s.write, s.read = w, r
	case "postgres":
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			return nil, fmt.Errorf("opening postgres: %w", err)
		}
		if maxConns < 1 {
			maxConns = 16
		}
		db.SetMaxOpenConns(maxConns)
		db.SetConnMaxLifetime(30 * time.Minute)
		s.write, s.read = db, db
	default:
		return nil, fmt.Errorf("unknown db driver %q", driver)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.write.PingContext(ctx); err != nil {
		s.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	if driver == "sqlite" {
		var ok string
		if err := s.write.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&ok); err == nil && ok != "ok" {
			s.Close()
			return nil, fmt.Errorf("sqlite database failed integrity check: %s", ok)
		}
	}

	if err := s.migrate(ctx); err != nil {
		s.Close()
		return nil, fmt.Errorf("migrating database: %w", err)
	}
	return s, nil
}

func (s *Store) acquireLock(dataDir string) error {
	path := filepath.Join(dataDir, "shortr.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("another shortr instance appears to be running against %s (lock file %s exists; remove it if that's not the case)", dataDir, path)
		}
		return fmt.Errorf("creating lock file: %w", err)
	}
	fmt.Fprintf(f, "%d", os.Getpid())
	s.lockFile = f
	return nil
}

func (s *Store) Close() error {
	if s.lockFile != nil {
		name := s.lockFile.Name()
		s.lockFile.Close()
		os.Remove(name)
	}
	if s.read != nil && s.read != s.write {
		s.read.Close()
	}
	if s.write != nil {
		return s.write.Close()
	}
	return nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.read.PingContext(ctx)
}

// q rewrites `?` placeholders to `$1, $2, ...` for postgres; no-op for sqlite.
func (s *Store) q(query string) string {
	if s.Driver != "postgres" {
		return query
	}
	var sb strings.Builder
	n := 0
	for _, c := range query {
		if c == '?' {
			n++
			sb.WriteByte('$')
			sb.WriteString(fmt.Sprint(n))
		} else {
			sb.WriteRune(c)
		}
	}
	return sb.String()
}

func (s *Store) execWrite(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.write.ExecContext(ctx, s.q(query), args...)
}

func (s *Store) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return s.read.QueryRowContext(ctx, s.q(query), args...)
}

func (s *Store) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.read.QueryContext(ctx, s.q(query), args...)
}

// WithTx runs fn in a write transaction. All mutating store methods that
// need atomicity use this; it always goes through the single write pool.
func (s *Store) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) txExec(tx *sql.Tx, ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tx.ExecContext(ctx, s.q(query), args...)
}

func (s *Store) txQueryRow(tx *sql.Tx, ctx context.Context, query string, args ...any) *sql.Row {
	return tx.QueryRowContext(ctx, s.q(query), args...)
}

// ErrNotFound is returned by single-row lookups that find nothing.
var ErrNotFound = errors.New("not found")

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }

// --- migrations -------------------------------------------------------

func (s *Store) migrate(ctx context.Context) error {
	createTbl := "CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)"
	if _, err := s.write.ExecContext(ctx, createTbl); err != nil {
		return err
	}

	if s.Driver == "postgres" {
		// advisory lock so multiple replicas don't migrate concurrently
		if _, err := s.write.ExecContext(ctx, "SELECT pg_advisory_lock(848301)"); err != nil {
			return err
		}
		defer s.write.ExecContext(ctx, "SELECT pg_advisory_unlock(848301)") //nolint:errcheck
	}

	fsys := sqliteMigrations
	dir := "migrations/sqlite"
	if s.Driver == "postgres" {
		fsys = postgresMigrations
		dir = "migrations/postgres"
	}
	entries, err := fsys.ReadDir(dir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	applied := map[int]bool{}
	rows, err := s.write.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return err
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		var version int
		if _, err := fmt.Sscanf(e.Name(), "%04d_", &version); err != nil {
			return fmt.Errorf("migration filename %q must start with a 4-digit version", e.Name())
		}
		if applied[version] {
			continue
		}
		content, err := fsys.ReadFile(dir + "/" + e.Name())
		if err != nil {
			return err
		}
		if err := s.applyMigration(ctx, version, string(content)); err != nil {
			return fmt.Errorf("migration %s: %w", e.Name(), err)
		}
	}
	return nil
}

func (s *Store) applyMigration(ctx context.Context, version int, sqlText string) error {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	for _, stmt := range splitStatements(sqlText) {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, s.q("INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)"), version, time.Now().UnixMilli()); err != nil {
		return err
	}
	return tx.Commit()
}

// splitStatements splits a .sql file on semicolons at statement boundaries.
// Our migration files never contain semicolons inside string literals, so a
// naive split is sufficient (ponytail: no real SQL parser needed here).
func splitStatements(s string) []string {
	return strings.Split(s, ";")
}
