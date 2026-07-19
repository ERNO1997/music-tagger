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
	id                INTEGER PRIMARY KEY AUTOINCREMENT,
	path              TEXT NOT NULL UNIQUE,
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
	if err := migratePrimaryKey(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating primary key: %w", err)
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
	{"lyrics", "TEXT NOT NULL DEFAULT ''"},
	{"synced_lyrics", "TEXT NOT NULL DEFAULT ''"},
	{"tagged", "INTEGER NOT NULL DEFAULT 0"},
	{"tag_error", "TEXT NOT NULL DEFAULT ''"},
	{"relocated", "INTEGER NOT NULL DEFAULT 0"},
	{"relocate_error", "TEXT NOT NULL DEFAULT ''"},
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

// columnIsPrimaryKey reports whether the named column is (part of) the
// files table's declared primary key.
func columnIsPrimaryKey(ctx context.Context, db *sql.DB, column string) (bool, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(files)")
	if err != nil {
		return false, fmt.Errorf("reading table info: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return false, fmt.Errorf("scanning table info row: %w", err)
		}
		if name == column {
			return pk != 0, nil
		}
	}
	return false, rows.Err()
}

// migratePrimaryKey rebuilds the files table if path is still its declared
// primary key (the shape used before relocation was added). SQLite can't
// alter an existing table's primary key in place, so this does a one-time
// create-copy-drop-rename inside a transaction. It's a no-op against a
// database that already has the surrogate-id shape, including a freshly
// created one (which gets that shape directly from the schema constant).
// migrateColumns must run first, so every column referenced here already
// exists on the old table regardless of which prior version it started from.
func migratePrimaryKey(ctx context.Context, db *sql.DB) error {
	pathIsPK, err := columnIsPrimaryKey(ctx, db, "path")
	if err != nil {
		return err
	}
	if !pathIsPK {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning primary-key migration transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE files_new (
			id                 INTEGER PRIMARY KEY AUTOINCREMENT,
			path               TEXT NOT NULL UNIQUE,
			format             TEXT NOT NULL,
			fingerprint        TEXT NOT NULL,
			duration_seconds   REAL NOT NULL DEFAULT 0,
			size               INTEGER NOT NULL,
			mtime              INTEGER NOT NULL,
			status             TEXT NOT NULL,
			missing            INTEGER NOT NULL DEFAULT 0,
			fingerprint_error  TEXT NOT NULL DEFAULT '',
			artist             TEXT NOT NULL DEFAULT '',
			album              TEXT NOT NULL DEFAULT '',
			title              TEXT NOT NULL DEFAULT '',
			track_number       INTEGER NOT NULL DEFAULT 0,
			recording_mbid     TEXT NOT NULL DEFAULT '',
			album_artist       TEXT NOT NULL DEFAULT '',
			year               INTEGER NOT NULL DEFAULT 0,
			disc_number        INTEGER NOT NULL DEFAULT 0,
			total_discs        INTEGER NOT NULL DEFAULT 0,
			total_tracks       INTEGER NOT NULL DEFAULT 0,
			release_mbid       TEXT NOT NULL DEFAULT '',
			release_group_mbid TEXT NOT NULL DEFAULT '',
			artist_mbid        TEXT NOT NULL DEFAULT '',
			cover_art_path     TEXT NOT NULL DEFAULT '',
			lyrics             TEXT NOT NULL DEFAULT '',
			synced_lyrics      TEXT NOT NULL DEFAULT '',
			tagged             INTEGER NOT NULL DEFAULT 0,
			tag_error          TEXT NOT NULL DEFAULT '',
			relocated          INTEGER NOT NULL DEFAULT 0,
			relocate_error     TEXT NOT NULL DEFAULT '',
			updated_at         INTEGER NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("creating rebuilt files table: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO files_new (
			path, format, fingerprint, duration_seconds, size, mtime, status, missing, fingerprint_error,
			artist, album, title, track_number, recording_mbid,
			album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid,
			cover_art_path, lyrics, synced_lyrics, tagged, tag_error, relocated, relocate_error, updated_at
		)
		SELECT
			path, format, fingerprint, duration_seconds, size, mtime, status, missing, fingerprint_error,
			artist, album, title, track_number, recording_mbid,
			album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid,
			cover_art_path, lyrics, synced_lyrics, tagged, tag_error, relocated, relocate_error, updated_at
		FROM files
	`); err != nil {
		return fmt.Errorf("copying rows into rebuilt files table: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE files`); err != nil {
		return fmt.Errorf("dropping old files table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE files_new RENAME TO files`); err != nil {
		return fmt.Errorf("renaming rebuilt files table: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing primary-key migration: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) LoadAll(ctx context.Context) (map[string]domain.FileRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT path, format, fingerprint, duration_seconds, size, mtime, status, missing, fingerprint_error,
		       artist, album, title, track_number, recording_mbid,
		       album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid,
		       cover_art_path, lyrics, synced_lyrics, tagged, tag_error, relocated, relocate_error
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
		var missing, tagged, relocated int
		if err := rows.Scan(&rec.Path, &format, &rec.Fingerprint, &rec.DurationSeconds, &rec.Size, &rec.ModTime, &status, &missing, &rec.FingerprintError,
			&rec.Artist, &rec.Album, &rec.Title, &rec.TrackNumber, &rec.RecordingMBID,
			&rec.AlbumArtist, &rec.Year, &rec.DiscNumber, &rec.TotalDiscs, &rec.TotalTracks, &rec.ReleaseMBID, &rec.ReleaseGroupMBID, &rec.ArtistMBID,
			&rec.CoverArtPath, &rec.Lyrics, &rec.SyncedLyrics, &tagged, &rec.TagError, &relocated, &rec.RelocateError); err != nil {
			return nil, fmt.Errorf("scanning tracked file row: %w", err)
		}
		rec.Format = domain.Format(format)
		rec.Status = domain.TrackingStatus(status)
		rec.Missing = missing != 0
		rec.Tagged = tagged != 0
		rec.Relocated = relocated != 0
		result[rec.Path] = rec
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading tracked file rows: %w", err)
	}

	return result, nil
}

// Get returns one file's complete tracked record via a single indexed
// lookup, for tagging without loading the whole table.
func (s *SQLiteStore) Get(ctx context.Context, path string) (domain.FileRecord, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT path, format, fingerprint, duration_seconds, size, mtime, status, missing, fingerprint_error,
		       artist, album, title, track_number, recording_mbid,
		       album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid,
		       cover_art_path, lyrics, synced_lyrics, tagged, tag_error, relocated, relocate_error
		FROM files
		WHERE path = ?
	`, path)

	var rec domain.FileRecord
	var format, status string
	var missing, tagged, relocated int
	err := row.Scan(&rec.Path, &format, &rec.Fingerprint, &rec.DurationSeconds, &rec.Size, &rec.ModTime, &status, &missing, &rec.FingerprintError,
		&rec.Artist, &rec.Album, &rec.Title, &rec.TrackNumber, &rec.RecordingMBID,
		&rec.AlbumArtist, &rec.Year, &rec.DiscNumber, &rec.TotalDiscs, &rec.TotalTracks, &rec.ReleaseMBID, &rec.ReleaseGroupMBID, &rec.ArtistMBID,
		&rec.CoverArtPath, &rec.Lyrics, &rec.SyncedLyrics, &tagged, &rec.TagError, &relocated, &rec.RelocateError)
	if err == sql.ErrNoRows {
		return domain.FileRecord{}, false, nil
	}
	if err != nil {
		return domain.FileRecord{}, false, fmt.Errorf("querying tracked record for %s: %w", path, err)
	}
	rec.Format = domain.Format(format)
	rec.Status = domain.TrackingStatus(status)
	rec.Missing = missing != 0
	rec.Tagged = tagged != 0
	rec.Relocated = relocated != 0
	return rec, true, nil
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
			INSERT INTO files (path, format, fingerprint, duration_seconds, size, mtime, status, missing, fingerprint_error, artist, album, title, track_number, recording_mbid, album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid, cover_art_path, lyrics, synced_lyrics, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, '', '', '', 0, '', '', 0, 0, 0, 0, '', '', '', '', '', '', ?)
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
				lyrics = '',
				synced_lyrics = '',
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
		// cover_art_path, lyrics/synced_lyrics, tagged/tag_error, and
		// relocated/relocate_error are reset here too: a re-identification
		// can resolve to a different release/recording than before, which
		// would make any previously stored enrichment, on-disk tagging, or
		// relocation outcome stale.
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
				lyrics = '',
				synced_lyrics = '',
				tagged = 0,
				tag_error = '',
				relocated = 0,
				relocate_error = '',
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

	_, err := s.db.ExecContext(ctx, `UPDATE files SET status = ?, cover_art_path = '', lyrics = '', synced_lyrics = '', tagged = 0, tag_error = '', relocated = 0, relocate_error = '', updated_at = ? WHERE path = ?`, string(result.Status), now, path)
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

// RecordLyrics updates one file's stored plain and synced lyrics, without
// altering its fingerprint, status, or resolved metadata.
func (s *SQLiteStore) RecordLyrics(ctx context.Context, path string, lyrics, syncedLyrics string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `UPDATE files SET lyrics = ?, synced_lyrics = ?, updated_at = ? WHERE path = ?`, lyrics, syncedLyrics, now, path)
	if err != nil {
		return fmt.Errorf("recording lyrics for %s: %w", path, err)
	}
	return nil
}

// GetLyrics returns one file's stored plain and synced lyrics via a single
// indexed lookup, for serving lyrics without loading the whole table.
func (s *SQLiteStore) GetLyrics(ctx context.Context, path string) (string, string, bool, error) {
	var lyrics, syncedLyrics string
	err := s.db.QueryRowContext(ctx, `SELECT lyrics, synced_lyrics FROM files WHERE path = ?`, path).Scan(&lyrics, &syncedLyrics)
	if err == sql.ErrNoRows {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, fmt.Errorf("querying lyrics for %s: %w", path, err)
	}
	return lyrics, syncedLyrics, lyrics != "" || syncedLyrics != "", nil
}

// RecordTagged updates one file's tagged outcome, without altering its
// fingerprint, status, resolved metadata, cover art path, or lyrics.
func (s *SQLiteStore) RecordTagged(ctx context.Context, path string, tagged bool, tagErr string) error {
	now := time.Now().Unix()
	taggedInt := 0
	if tagged {
		taggedInt = 1
	}
	_, err := s.db.ExecContext(ctx, `UPDATE files SET tagged = ?, tag_error = ?, updated_at = ? WHERE path = ?`, taggedInt, tagErr, now, path)
	if err != nil {
		return fmt.Errorf("recording tagged outcome for %s: %w", path, err)
	}
	return nil
}

// RecordFileStat updates one file's stored size and modification time,
// without altering any other field.
func (s *SQLiteStore) RecordFileStat(ctx context.Context, path string, size int64, modTime int64) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `UPDATE files SET size = ?, mtime = ?, updated_at = ? WHERE path = ?`, size, modTime, now, path)
	if err != nil {
		return fmt.Errorf("recording file stat for %s: %w", path, err)
	}
	return nil
}

// RecordRelocation updates one file's path to its new, post-relocation
// location and marks it relocated, in a single commit, without altering
// any other field. It identifies the row by its old path value, same as
// every other per-file update — unaffected by path no longer being the
// declared primary key.
func (s *SQLiteStore) RecordRelocation(ctx context.Context, oldPath, newPath string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `UPDATE files SET path = ?, relocated = 1, relocate_error = '', updated_at = ? WHERE path = ?`, newPath, now, oldPath)
	if err != nil {
		return fmt.Errorf("recording relocation of %s to %s: %w", oldPath, newPath, err)
	}
	return nil
}

// RecordRelocationFailure updates one file's relocation outcome to failed
// with the given reason, without altering its path or any other field.
func (s *SQLiteStore) RecordRelocationFailure(ctx context.Context, path string, relocateErr string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `UPDATE files SET relocated = 0, relocate_error = ?, updated_at = ? WHERE path = ?`, relocateErr, now, path)
	if err != nil {
		return fmt.Errorf("recording relocation failure for %s: %w", path, err)
	}
	return nil
}
