# Backup & Restore

## SQLite (default)

**Automatic:** every `SHORTR_BACKUP_INTERVAL` (default `24h`), Shortr runs a
live, consistent `VACUUM INTO` backup — safe to run while the server is
serving traffic — into `<DATA_DIR>/backups/`, and keeps the most recent
`SHORTR_BACKUP_KEEP` (default `7`). If the backup fails, admins are notified
(in-app, and by email/Gotify if configured).

**On-demand:**

```bash
shortr backup                      # writes to <data_dir>/backups/manual-<timestamp>.db
shortr backup --out /path/to/file.db
```

Or trigger one via the API: `POST /api/v1/admin/backup` (admin only).

**Restoring:** stop Shortr, replace `<DATA_DIR>/shortr.db` with the backup
file, start Shortr again.

```bash
sudo systemctl stop shortr
cp /var/lib/shortr/backups/manual-20260115T030000.db /var/lib/shortr/shortr.db
sudo systemctl start shortr
```

`shortr migrate` will apply any migrations newer than the backup
automatically on next start.

## PostgreSQL

The built-in backup job only supports SQLite (`SHORTR_BACKUP_INTERVAL` is a
no-op — logged as a warning at startup — when `SHORTR_DB_DRIVER=postgres`).
Use standard Postgres tooling instead:

```bash
pg_dump "$SHORTR_DB_DSN" -Fc -f shortr-$(date +%Y%m%d).dump
# restore:
pg_restore -d "$SHORTR_DB_DSN" --clean shortr-20260115.dump
```

If you run Postgres in Docker via
`deploy/docker-compose.postgres.yml`, prefer volume snapshots or your
Postgres provider's managed backup feature for point-in-time recovery.

## What's *not* in a backup

- The `secret.key` file (if auto-generated rather than set via
  `SHORTR_SECRET_KEY`) — back it up separately, or losing it invalidates all
  existing sessions/API keys on restore to a different host.
- The GeoIP `.mmdb` file, if configured — it's a static, re-downloadable
  asset, not user data.
- The click spool directory (`<DATA_DIR>/clicks-spool/`) — transient,
  in-flight click events waiting to be replayed into the DB after an outage;
  safe to lose a few seconds of it, but don't delete it while the server is
  running.
