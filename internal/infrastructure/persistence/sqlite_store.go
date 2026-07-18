package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"music-tagger/internal/domain"
	"music-tagger/internal/usecases"
)

const schema = `
CREATE TABLE IF NOT EXISTS files (
	path              TEXT PRIMARY KEY,
	format            TEXT NOT NULL,
	fingerprint       TEXT NOT NULL,
	duration_seconds  REAL NOT NULL DEFAULT 0,
	size              INTEGER NOT NULL,
	mtime             INTEGER NOT NULL,
	status            TEXT NOT NULL,
	missing           INTEGER NOT NULL DEFAULT 0,
	fingerprint_error TEXT NOT NULL DEFAULT '',
	updated_at        INTEGER NOT NULL
);
`

// SQLiteStore is a TrackingStore backed by an embedded SQLite database
// (pure-Go driver, no CGO).
type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(ctx context.Context, dbPath string) (*SQLiteStore, error) {
	if dir := filepath.Dir(dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating db directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database: %w", err)
	}

	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enabling WAL mode: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA synchronous=NORMAL;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting synchronous mode: %w", err)
	}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) LoadAll(ctx context.Context) (map[string]domain.FileRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT path, format, fingerprint, duration_seconds, size, mtime, status, missing, fingerprint_error FROM files
	`)
	if err != nil {
		return nil, fmt.Errorf("querying tracked files: %w", err)
	}
	defer rows.Close()

	result := make(map[string]domain.FileRecord)
	for rows.Next() {
		var rec domain.FileRecord
		var format, status string
		var missing int
		if err := rows.Scan(&rec.Path, &format, &rec.Fingerprint, &rec.DurationSeconds, &rec.Size, &rec.ModTime, &status, &missing, &rec.FingerprintError); err != nil {
			return nil, fmt.Errorf("scanning tracked file row: %w", err)
		}
		rec.Format = domain.Format(format)
		rec.Status = domain.TrackingStatus(status)
		rec.Missing = missing != 0
		result[rec.Path] = rec
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading tracked file rows: %w", err)
	}

	return result, nil
}

func (s *SQLiteStore) BulkApply(ctx context.Context, apply usecases.BulkApply) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().Unix()

	if len(apply.Upserts) > 0 {
		upsertStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO files (path, format, fingerprint, duration_seconds, size, mtime, status, missing, fingerprint_error, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, ?)
			ON CONFLICT(path) DO UPDATE SET
				format = excluded.format,
				fingerprint = excluded.fingerprint,
				duration_seconds = excluded.duration_seconds,
				size = excluded.size,
				mtime = excluded.mtime,
				status = excluded.status,
				missing = 0,
				fingerprint_error = excluded.fingerprint_error,
				updated_at = excluded.updated_at
		`)
		if err != nil {
			return fmt.Errorf("preparing upsert statement: %w", err)
		}
		defer upsertStmt.Close()

		for _, rec := range apply.Upserts {
			if _, err := upsertStmt.ExecContext(ctx, rec.Path, string(rec.Format), rec.Fingerprint, rec.DurationSeconds, rec.Size, rec.ModTime, string(rec.Status), rec.FingerprintError, now); err != nil {
				return fmt.Errorf("upserting %s: %w", rec.Path, err)
			}
		}
	}

	if len(apply.MissingPaths) > 0 {
		missingStmt, err := tx.PrepareContext(ctx, `UPDATE files SET missing = 1, updated_at = ? WHERE path = ?`)
		if err != nil {
			return fmt.Errorf("preparing missing statement: %w", err)
		}
		defer missingStmt.Close()

		for _, path := range apply.MissingPaths {
			if _, err := missingStmt.ExecContext(ctx, now, path); err != nil {
				return fmt.Errorf("marking %s missing: %w", path, err)
			}
		}
	}

	if len(apply.ReappearedPaths) > 0 {
		reappearStmt, err := tx.PrepareContext(ctx, `UPDATE files SET missing = 0, updated_at = ? WHERE path = ?`)
		if err != nil {
			return fmt.Errorf("preparing reappeared statement: %w", err)
		}
		defer reappearStmt.Close()

		for _, path := range apply.ReappearedPaths {
			if _, err := reappearStmt.ExecContext(ctx, now, path); err != nil {
				return fmt.Errorf("restoring %s: %w", path, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing refresh: %w", err)
	}

	return nil
}
