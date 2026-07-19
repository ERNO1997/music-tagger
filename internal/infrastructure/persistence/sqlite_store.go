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
	artist            TEXT NOT NULL DEFAULT '',
	album             TEXT NOT NULL DEFAULT '',
	title             TEXT NOT NULL DEFAULT '',
	track_number      INTEGER NOT NULL DEFAULT 0,
	recording_mbid    TEXT NOT NULL DEFAULT '',
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
	if err := migrateColumns(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating schema: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// columnMigrations lists columns added to `files` after its initial release,
// for databases created before each column existed. CREATE TABLE IF NOT
// EXISTS is a no-op against an already-existing table, so new columns must
// be added explicitly — this makes that idempotent regardless of which
// prior schema version a given database started from.
var columnMigrations = []struct {
	name       string
	definition string
}{
	{"artist", "TEXT NOT NULL DEFAULT ''"},
	{"album", "TEXT NOT NULL DEFAULT ''"},
	{"title", "TEXT NOT NULL DEFAULT ''"},
	{"track_number", "INTEGER NOT NULL DEFAULT 0"},
	{"recording_mbid", "TEXT NOT NULL DEFAULT ''"},
	{"album_artist", "TEXT NOT NULL DEFAULT ''"},
	{"year", "INTEGER NOT NULL DEFAULT 0"},
	{"disc_number", "INTEGER NOT NULL DEFAULT 0"},
	{"total_discs", "INTEGER NOT NULL DEFAULT 0"},
	{"total_tracks", "INTEGER NOT NULL DEFAULT 0"},
	{"release_mbid", "TEXT NOT NULL DEFAULT ''"},
	{"release_group_mbid", "TEXT NOT NULL DEFAULT ''"},
	{"artist_mbid", "TEXT NOT NULL DEFAULT ''"},
	{"cover_art_path", "TEXT NOT NULL DEFAULT ''"},
}

func migrateColumns(ctx context.Context, db *sql.DB) error {
	existing, err := existingColumns(ctx, db)
	if err != nil {
		return err
	}

	for _, col := range columnMigrations {
		if existing[col.name] {
			continue
		}
		stmt := fmt.Sprintf("ALTER TABLE files ADD COLUMN %s %s", col.name, col.definition)
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("adding column %s: %w", col.name, err)
		}
	}

	return nil
}

func existingColumns(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(files)")
	if err != nil {
		return nil, fmt.Errorf("reading table info: %w", err)
	}
	defer rows.Close()

	existing := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return nil, fmt.Errorf("scanning table info row: %w", err)
		}
		existing[name] = true
	}
	return existing, rows.Err()
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) LoadAll(ctx context.Context) (map[string]domain.FileRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT path, format, fingerprint, duration_seconds, size, mtime, status, missing, fingerprint_error,
		       artist, album, title, track_number, recording_mbid,
		       album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid,
		       cover_art_path
		FROM files
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
		if err := rows.Scan(&rec.Path, &format, &rec.Fingerprint, &rec.DurationSeconds, &rec.Size, &rec.ModTime, &status, &missing, &rec.FingerprintError,
			&rec.Artist, &rec.Album, &rec.Title, &rec.TrackNumber, &rec.RecordingMBID,
			&rec.AlbumArtist, &rec.Year, &rec.DiscNumber, &rec.TotalDiscs, &rec.TotalTracks, &rec.ReleaseMBID, &rec.ReleaseGroupMBID, &rec.ArtistMBID,
			&rec.CoverArtPath); err != nil {
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
		// Resolved metadata (artist/album/title/track_number/recording_mbid
		// and the extended fields below) is always reset to blank here —
		// every Upserts entry is a brand new or content-changed file, so
		// any prior identification is stale and must not linger against
		// different content.
		upsertStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO files (path, format, fingerprint, duration_seconds, size, mtime, status, missing, fingerprint_error, artist, album, title, track_number, recording_mbid, album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid, cover_art_path, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, '', '', '', 0, '', '', 0, 0, 0, 0, '', '', '', '', ?)
			ON CONFLICT(path) DO UPDATE SET
				format = excluded.format,
				fingerprint = excluded.fingerprint,
				duration_seconds = excluded.duration_seconds,
				size = excluded.size,
				mtime = excluded.mtime,
				status = excluded.status,
				missing = 0,
				fingerprint_error = excluded.fingerprint_error,
				artist = '',
				album = '',
				title = '',
				track_number = 0,
				recording_mbid = '',
				album_artist = '',
				year = 0,
				disc_number = 0,
				total_discs = 0,
				total_tracks = 0,
				release_mbid = '',
				release_group_mbid = '',
				artist_mbid = '',
				cover_art_path = '',
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

// RecordIdentification updates one file's status and, when identified, its
// resolved metadata — committed immediately (not batched), since each
// identify iteration is already paced to ~1 req/sec by the MusicBrainz rate
// gate and a single commit is negligible next to that.
func (s *SQLiteStore) RecordIdentification(ctx context.Context, path string, result usecases.IdentificationResult) error {
	now := time.Now().Unix()

	if result.Status == domain.StatusIdentified {
		// cover_art_path is reset here too: a re-identification can resolve
		// to a different release than before, which would make any
		// previously stored cover art stale.
		_, err := s.db.ExecContext(ctx, `
			UPDATE files SET
				status = ?,
				artist = ?,
				album = ?,
				title = ?,
				track_number = ?,
				recording_mbid = ?,
				album_artist = ?,
				year = ?,
				disc_number = ?,
				total_discs = ?,
				total_tracks = ?,
				release_mbid = ?,
				release_group_mbid = ?,
				artist_mbid = ?,
				cover_art_path = '',
				updated_at = ?
			WHERE path = ?
		`, string(result.Status), result.Metadata.Artist, result.Metadata.Album, result.Metadata.Title,
			result.Metadata.TrackNumber, result.Metadata.RecordingID,
			result.Metadata.AlbumArtist, result.Metadata.Year, result.Metadata.DiscNumber,
			result.Metadata.TotalDiscs, result.Metadata.TotalTracks,
			result.Metadata.ReleaseMBID, result.Metadata.ReleaseGroupMBID, result.Metadata.ArtistMBID,
			now, path)
		if err != nil {
			return fmt.Errorf("recording identification for %s: %w", path, err)
		}
		return nil
	}

	_, err := s.db.ExecContext(ctx, `UPDATE files SET status = ?, cover_art_path = '', updated_at = ? WHERE path = ?`, string(result.Status), now, path)
	if err != nil {
		return fmt.Errorf("recording identification outcome for %s: %w", path, err)
	}
	return nil
}

// RecordCoverArt updates one file's stored cover art path, without
// altering its fingerprint, status, or resolved metadata.
func (s *SQLiteStore) RecordCoverArt(ctx context.Context, path string, coverArtPath string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `UPDATE files SET cover_art_path = ?, updated_at = ? WHERE path = ?`, coverArtPath, now, path)
	if err != nil {
		return fmt.Errorf("recording cover art for %s: %w", path, err)
	}
	return nil
}

// GetCoverArtPath returns one file's stored cover art path via a single
// indexed lookup, for serving cover images without loading the whole table.
func (s *SQLiteStore) GetCoverArtPath(ctx context.Context, path string) (string, bool, error) {
	var coverArtPath string
	err := s.db.QueryRowContext(ctx, `SELECT cover_art_path FROM files WHERE path = ?`, path).Scan(&coverArtPath)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("querying cover art path for %s: %w", path, err)
	}
	return coverArtPath, coverArtPath != "", nil
}
