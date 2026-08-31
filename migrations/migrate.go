package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed *.up.sql
var files embed.FS

const migrationLockName = "orion_live_schema_migrations"

func Up(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Close()

	var acquired int
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 10)", migrationLockName).Scan(&acquired); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	if acquired != 1 {
		return fmt.Errorf("migration lock %q is held by another process", migrationLockName)
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", migrationLockName)

	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
        version BIGINT UNSIGNED NOT NULL PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        checksum CHAR(64) NOT NULL,
        applied_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	migrationFiles, err := fs.Glob(files, "*.up.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(migrationFiles)
	for _, name := range migrationFiles {
		if err := apply(ctx, conn, name); err != nil {
			return err
		}
	}
	return nil
}

func apply(ctx context.Context, conn *sql.Conn, name string) error {
	versionText, _, ok := strings.Cut(name, "_")
	if !ok {
		return fmt.Errorf("invalid migration filename %q", name)
	}
	version, err := strconv.ParseUint(versionText, 10, 64)
	if err != nil {
		return fmt.Errorf("parse migration version %q: %w", name, err)
	}
	content, err := files.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read migration %q: %w", name, err)
	}
	digest := sha256.Sum256(content)
	checksum := hex.EncodeToString(digest[:])

	var storedChecksum string
	err = conn.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE version = ?", version).Scan(&storedChecksum)
	if err == nil {
		if storedChecksum != checksum {
			return fmt.Errorf("migration %q was modified after being applied", name)
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read migration state for %q: %w", name, err)
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %q: %w", name, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("execute migration %q: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, name, checksum) VALUES (?, ?, ?)", version, name, checksum); err != nil {
		return fmt.Errorf("record migration %q: %w", name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %q: %w", name, err)
	}
	return nil
}
