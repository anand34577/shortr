package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"shortr/internal/auth"
	"shortr/internal/config"
	"shortr/internal/store"
)

func loadCfgOrExit() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(2)
	}
	return cfg
}

func cmdMigrate(args []string) {
	cfg := loadCfgOrExit()
	st, err := store.Open(cfg.DBDriver, cfg.DBDSN, cfg.DataDir, cfg.DBMaxConns)
	if err != nil {
		fmt.Fprintln(os.Stderr, "migration failed:", err)
		os.Exit(1)
	}
	defer st.Close()
	fmt.Println("database is up to date")
}

func cmdAdmin(args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: shortr admin <create|reset-password|promote> [flags]")
		os.Exit(2)
	}
	sub := args[0]
	rest := args[1:]
	cfg := loadCfgOrExit()
	st, err := store.Open(cfg.DBDriver, cfg.DBDSN, cfg.DataDir, cfg.DBMaxConns)
	if err != nil {
		fmt.Fprintln(os.Stderr, "database error:", err)
		os.Exit(1)
	}
	defer st.Close()
	ctx := context.Background()

	switch sub {
	case "create":
		fs := flag.NewFlagSet("admin create", flag.ExitOnError)
		email := fs.String("email", "", "admin email (required)")
		password := fs.String("password", "", "admin password (generated if omitted)")
		name := fs.String("name", "Admin", "display name")
		fs.Parse(rest) //nolint:errcheck
		if *email == "" {
			fmt.Fprintln(os.Stderr, "--email is required")
			os.Exit(2)
		}
		pw := *password
		generated := pw == ""
		if generated {
			pw, _, _ = auth.NewOpaqueToken(18)
		}
		hash, err := auth.HashPassword(pw)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		now := time.Now()
		u := &store.User{Email: *email, Name: *name, PasswordHash: &hash, Role: "admin", Status: "active", EmailVerified: true, RoleLocked: true, PasswordChangedAt: &now}
		if err := st.CreateUser(ctx, u); err != nil {
			fmt.Fprintln(os.Stderr, "error creating user:", err)
			os.Exit(1)
		}
		fmt.Println("admin user created:", *email)
		if generated {
			fmt.Println("generated password:", pw)
		}

	case "reset-password":
		fs := flag.NewFlagSet("admin reset-password", flag.ExitOnError)
		email := fs.String("email", "", "user email (required)")
		password := fs.String("password", "", "new password (generated if omitted)")
		fs.Parse(rest) //nolint:errcheck
		if *email == "" {
			fmt.Fprintln(os.Stderr, "--email is required")
			os.Exit(2)
		}
		u, err := st.GetUserByEmail(ctx, *email)
		if err != nil {
			fmt.Fprintln(os.Stderr, "user not found:", *email)
			os.Exit(1)
		}
		pw := *password
		generated := pw == ""
		if generated {
			pw, _, _ = auth.NewOpaqueToken(18)
		}
		hash, err := auth.HashPassword(pw)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		now := time.Now()
		u.PasswordHash = &hash
		u.PasswordChangedAt = &now
		if err := st.UpdateUser(ctx, u); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		_ = st.DeleteSessionsForUser(ctx, u.ID)
		fmt.Println("password reset for", *email)
		if generated {
			fmt.Println("generated password:", pw)
		}

	case "promote":
		fs := flag.NewFlagSet("admin promote", flag.ExitOnError)
		email := fs.String("email", "", "user email (required)")
		fs.Parse(rest) //nolint:errcheck
		if *email == "" {
			fmt.Fprintln(os.Stderr, "--email is required")
			os.Exit(2)
		}
		u, err := st.GetUserByEmail(ctx, *email)
		if err != nil {
			fmt.Fprintln(os.Stderr, "user not found:", *email)
			os.Exit(1)
		}
		u.Role = "admin"
		u.RoleLocked = true
		if err := st.UpdateUser(ctx, u); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Println("promoted", *email, "to admin")

	default:
		fmt.Fprintln(os.Stderr, "unknown subcommand:", sub)
		os.Exit(2)
	}
}

func cmdBackup(args []string) {
	fs := flag.NewFlagSet("backup", flag.ExitOnError)
	out := fs.String("out", "", "output path (default: <data_dir>/backups/manual-<timestamp>.db)")
	fs.Parse(args) //nolint:errcheck

	cfg := loadCfgOrExit()
	if cfg.DBDriver != "sqlite" {
		fmt.Fprintln(os.Stderr, "backup command only supports the sqlite driver; use pg_dump for postgres")
		os.Exit(2)
	}
	st, err := store.Open(cfg.DBDriver, cfg.DBDSN, cfg.DataDir, cfg.DBMaxConns)
	if err != nil {
		fmt.Fprintln(os.Stderr, "database error:", err)
		os.Exit(1)
	}
	defer st.Close()

	dest := *out
	if dest == "" {
		dir := filepath.Join(cfg.DataDir, "backups")
		os.MkdirAll(dir, 0o700) //nolint:errcheck
		dest = filepath.Join(dir, "manual-"+time.Now().UTC().Format("20060102T150405")+".db")
	}
	if err := st.BackupSQLite(context.Background(), dest); err != nil {
		fmt.Fprintln(os.Stderr, "backup failed:", err)
		os.Exit(1)
	}
	fmt.Println("backup written to", dest)
}
