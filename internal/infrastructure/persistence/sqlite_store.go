package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// candidatesSchema holds every distinct recording resolved from a top
// AcoustID result that ties multiple recordings to the same audio, for a
// file currently recorded StatusAmbiguous. Rows are replaced wholesale
// (never individually updated) each time a file is recorded ambiguous, and
// deleted wholesale once it's resolved or re-identified.
const candidatesSchema = `
CREATE TABLE IF NOT EXISTS identification_candidates (
	path               TEXT NOT NULL,
	recording_mbid     TEXT NOT NULL,
	artist             TEXT NOT NULL DEFAULT '',
	album              TEXT NOT NULL DEFAULT '',
	title              TEXT NOT NULL DEFAULT '',
	track_number       INTEGER NOT NULL DEFAULT 0,
	album_artist       TEXT NOT NULL DEFAULT '',
	year               INTEGER NOT NULL DEFAULT 0,
	disc_number        INTEGER NOT NULL DEFAULT 0,
	total_discs        INTEGER NOT NULL DEFAULT 0,
	total_tracks       INTEGER NOT NULL DEFAULT 0,
	release_mbid       TEXT NOT NULL DEFAULT '',
	release_group_mbid TEXT NOT NULL DEFAULT '',
	artist_mbid        TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (path, recording_mbid)
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

	// modernc.org/sqlite (this project's pure-Go, CGO-free driver) corrupts
	// a WAL-mode database's freelist when Go's database/sql pool hands out
	// more than one concurrent OS-level connection into the same file — as
	// this project's several independent background job goroutines
	// (scan/analysis/identify/enrich/tag/relocate) can all do against this
	// same *sql.DB. Capping the pool at one connection serializes every
	// query/exec through it, eliminating the concurrent-writer hazard.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

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
	if _, err := db.ExecContext(ctx, candidatesSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating candidates schema: %w", err)
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
	{"raw_title", "TEXT NOT NULL DEFAULT ''"},
	{"raw_artist", "TEXT NOT NULL DEFAULT ''"},
	{"raw_album", "TEXT NOT NULL DEFAULT ''"},
	{"raw_album_artist", "TEXT NOT NULL DEFAULT ''"},
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
			raw_title          TEXT NOT NULL DEFAULT '',
			raw_artist         TEXT NOT NULL DEFAULT '',
			raw_album          TEXT NOT NULL DEFAULT '',
			raw_album_artist   TEXT NOT NULL DEFAULT '',
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
			cover_art_path, lyrics, synced_lyrics, tagged, tag_error, relocated, relocate_error,
			raw_title, raw_artist, raw_album, raw_album_artist, updated_at
		)
		SELECT
			path, format, fingerprint, duration_seconds, size, mtime, status, missing, fingerprint_error,
			artist, album, title, track_number, recording_mbid,
			album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid,
			cover_art_path, lyrics, synced_lyrics, tagged, tag_error, relocated, relocate_error,
			raw_title, raw_artist, raw_album, raw_album_artist, updated_at
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
		       cover_art_path, lyrics, synced_lyrics, tagged, tag_error, relocated, relocate_error,
		       raw_title, raw_artist, raw_album, raw_album_artist
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
			&rec.CoverArtPath, &rec.Lyrics, &rec.SyncedLyrics, &tagged, &rec.TagError, &relocated, &rec.RelocateError,
			&rec.RawTitle, &rec.RawArtist, &rec.RawAlbum, &rec.RawAlbumArtist); err != nil {
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
		       cover_art_path, lyrics, synced_lyrics, tagged, tag_error, relocated, relocate_error,
		       raw_title, raw_artist, raw_album, raw_album_artist
		FROM files
		WHERE path = ?
	`, path)

	var rec domain.FileRecord
	var format, status string
	var missing, tagged, relocated int
	err := row.Scan(&rec.Path, &format, &rec.Fingerprint, &rec.DurationSeconds, &rec.Size, &rec.ModTime, &status, &missing, &rec.FingerprintError,
		&rec.Artist, &rec.Album, &rec.Title, &rec.TrackNumber, &rec.RecordingMBID,
		&rec.AlbumArtist, &rec.Year, &rec.DiscNumber, &rec.TotalDiscs, &rec.TotalTracks, &rec.ReleaseMBID, &rec.ReleaseGroupMBID, &rec.ArtistMBID,
		&rec.CoverArtPath, &rec.Lyrics, &rec.SyncedLyrics, &tagged, &rec.TagError, &relocated, &rec.RelocateError,
		&rec.RawTitle, &rec.RawArtist, &rec.RawAlbum, &rec.RawAlbumArtist)
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
		// different content. Raw tag fields, by contrast, are written from
		// this pass's freshly-read values (not reset to blank), since
		// they're independent of resolved metadata and reflect the file's
		// current content.
		upsertStmt, err := tx.PrepareContext(ctx, `
			INSERT INTO files (path, format, fingerprint, duration_seconds, size, mtime, status, missing, fingerprint_error, artist, album, title, track_number, recording_mbid, album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid, cover_art_path, lyrics, synced_lyrics, raw_title, raw_artist, raw_album, raw_album_artist, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, '', '', '', 0, '', '', 0, 0, 0, 0, '', '', '', '', '', '', ?, ?, ?, ?, ?)
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
				raw_title = excluded.raw_title,
				raw_artist = excluded.raw_artist,
				raw_album = excluded.raw_album,
				raw_album_artist = excluded.raw_album_artist,
				updated_at = excluded.updated_at
		`)
		if err != nil {
			return fmt.Errorf("preparing upsert statement: %w", err)
		}
		defer upsertStmt.Close()

		for _, rec := range apply.Upserts {
			if _, err := upsertStmt.ExecContext(ctx, rec.Path, string(rec.Format), rec.Fingerprint, rec.DurationSeconds, rec.Size, rec.ModTime, string(rec.Status), rec.FingerprintError, rec.RawTitle, rec.RawArtist, rec.RawAlbum, rec.RawAlbumArtist, now); err != nil {
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
		if err := s.clearCandidates(ctx, path); err != nil {
			return err
		}
		return nil
	}

	_, err := s.db.ExecContext(ctx, `UPDATE files SET status = ?, cover_art_path = '', lyrics = '', synced_lyrics = '', tagged = 0, tag_error = '', relocated = 0, relocate_error = '', updated_at = ? WHERE path = ?`, string(result.Status), now, path)
	if err != nil {
		return fmt.Errorf("recording identification outcome for %s: %w", path, err)
	}
	if err := s.clearCandidates(ctx, path); err != nil {
		return err
	}
	return nil
}

// clearCandidates deletes any stored candidates for path — called whenever
// a file is identified again (whether it resolves to identified, not_found,
// or ambiguous), so a stale candidate list from a prior ambiguous outcome
// never lingers past a fresh identification attempt.
func (s *SQLiteStore) clearCandidates(ctx context.Context, path string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM identification_candidates WHERE path = ?`, path); err != nil {
		return fmt.Errorf("clearing stale candidates for %s: %w", path, err)
	}
	return nil
}

// RecordAmbiguous replaces path's stored candidates with candidates and
// records it as ambiguous, clearing resolved metadata and invalidating
// enrichment/tagged/relocated outcomes in the same transaction, mirroring
// RecordIdentification's not-found branch.
func (s *SQLiteStore) RecordAmbiguous(ctx context.Context, path string, candidates []usecases.RecordingMetadata) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning ambiguous-recording transaction for %s: %w", path, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM identification_candidates WHERE path = ?`, path); err != nil {
		return fmt.Errorf("clearing prior candidates for %s: %w", path, err)
	}

	insertStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO identification_candidates (path, recording_mbid, artist, album, title, track_number, album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing candidate insert for %s: %w", path, err)
	}
	defer insertStmt.Close()

	for _, c := range candidates {
		if _, err := insertStmt.ExecContext(ctx, path, c.RecordingID, c.Artist, c.Album, c.Title, c.TrackNumber, c.AlbumArtist, c.Year, c.DiscNumber, c.TotalDiscs, c.TotalTracks, c.ReleaseMBID, c.ReleaseGroupMBID, c.ArtistMBID); err != nil {
			return fmt.Errorf("inserting candidate %s for %s: %w", c.RecordingID, path, err)
		}
	}

	now := time.Now().Unix()
	if _, err := tx.ExecContext(ctx, `
		UPDATE files SET
			status = ?,
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
			tagged = 0,
			tag_error = '',
			relocated = 0,
			relocate_error = '',
			updated_at = ?
		WHERE path = ?
	`, string(domain.StatusAmbiguous), now, path); err != nil {
		return fmt.Errorf("recording ambiguous outcome for %s: %w", path, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing ambiguous recording for %s: %w", path, err)
	}
	return nil
}

// GetCandidates returns path's stored candidate list via a single indexed
// lookup, for the details view's candidate picker.
func (s *SQLiteStore) GetCandidates(ctx context.Context, path string) ([]usecases.RecordingMetadata, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT recording_mbid, artist, album, title, track_number, album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid
		FROM identification_candidates
		WHERE path = ?
	`, path)
	if err != nil {
		return nil, fmt.Errorf("querying candidates for %s: %w", path, err)
	}
	defer rows.Close()

	var candidates []usecases.RecordingMetadata
	for rows.Next() {
		var c usecases.RecordingMetadata
		if err := rows.Scan(&c.RecordingID, &c.Artist, &c.Album, &c.Title, &c.TrackNumber, &c.AlbumArtist, &c.Year, &c.DiscNumber, &c.TotalDiscs, &c.TotalTracks, &c.ReleaseMBID, &c.ReleaseGroupMBID, &c.ArtistMBID); err != nil {
			return nil, fmt.Errorf("scanning candidate row for %s: %w", path, err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading candidate rows for %s: %w", path, err)
	}
	return candidates, nil
}

// ResolveAmbiguous records the stored candidate matching recordingMBID as
// path's resolved identification and discards its other stored candidates,
// in one transaction. found is false (with a nil error) when recordingMBID
// doesn't match any of path's stored candidates.
func (s *SQLiteStore) ResolveAmbiguous(ctx context.Context, path, recordingMBID string) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("beginning resolve transaction for %s: %w", path, err)
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		SELECT recording_mbid, artist, album, title, track_number, album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid
		FROM identification_candidates
		WHERE path = ? AND recording_mbid = ?
	`, path, recordingMBID)

	var c usecases.RecordingMetadata
	if err := row.Scan(&c.RecordingID, &c.Artist, &c.Album, &c.Title, &c.TrackNumber, &c.AlbumArtist, &c.Year, &c.DiscNumber, &c.TotalDiscs, &c.TotalTracks, &c.ReleaseMBID, &c.ReleaseGroupMBID, &c.ArtistMBID); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("querying candidate %s for %s: %w", recordingMBID, path, err)
	}

	now := time.Now().Unix()
	if _, err := tx.ExecContext(ctx, `
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
	`, string(domain.StatusIdentified), c.Artist, c.Album, c.Title, c.TrackNumber, c.RecordingID,
		c.AlbumArtist, c.Year, c.DiscNumber, c.TotalDiscs, c.TotalTracks,
		c.ReleaseMBID, c.ReleaseGroupMBID, c.ArtistMBID, now, path); err != nil {
		return false, fmt.Errorf("recording resolved identification for %s: %w", path, err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM identification_candidates WHERE path = ?`, path); err != nil {
		return false, fmt.Errorf("clearing candidates for %s: %w", path, err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("committing resolve for %s: %w", path, err)
	}
	return true, nil
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

// RecordFingerprint updates one file's fingerprint, duration, and
// fingerprint error, without altering its status, resolved metadata, or any
// other field. Called once by IdentifyFile.Identify after it computes (or
// fails to compute) a fingerprint on demand.
func (s *SQLiteStore) RecordFingerprint(ctx context.Context, path string, fingerprint string, durationSeconds float64, fingerprintErr string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `UPDATE files SET fingerprint = ?, duration_seconds = ?, fingerprint_error = ?, updated_at = ? WHERE path = ?`, fingerprint, durationSeconds, fingerprintErr, now, path)
	if err != nil {
		return fmt.Errorf("recording fingerprint for %s: %w", path, err)
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

// librarySortColumns is the allow-list from a public LibrarySort.By value to
// a literal SQL column name. User input must never be interpolated directly
// into an ORDER BY clause — SQL parameterization doesn't cover identifiers —
// so an unrecognized or empty By value falls back to "path".
var librarySortColumns = map[string]string{
	"path":     "path",
	"status":   "status",
	"artist":   "artist",
	"album":    "album",
	"duration": "duration_seconds",
	"year":     "year",
}

// buildLibraryWhere translates a LibraryFilter into a parameterized WHERE
// clause (without the "WHERE" keyword) shared by QueryPage and QueryPaths.
func buildLibraryWhere(filter usecases.LibraryFilter) (string, []any) {
	var clauses []string
	var args []any

	if len(filter.Paths) > 0 {
		placeholders := strings.Repeat("?,", len(filter.Paths))
		placeholders = placeholders[:len(placeholders)-1]
		args = append(args, make([]any, len(filter.Paths))...)
		for i, p := range filter.Paths {
			args[i] = p
		}
		return "path IN (" + placeholders + ")", args
	}

	if filter.Status != "" {
		if domain.TrackingStatus(filter.Status) == domain.StatusMissing {
			clauses = append(clauses, "missing = 1")
		} else {
			clauses = append(clauses, "missing = 0 AND status = ?")
			args = append(args, filter.Status)
		}
	}
	if filter.Tagged != nil {
		clauses = append(clauses, "tagged = ?")
		args = append(args, boolToInt(*filter.Tagged))
	}
	if filter.Relocated != nil {
		clauses = append(clauses, "relocated = ?")
		args = append(args, boolToInt(*filter.Relocated))
	}
	if filter.HasLyrics != nil {
		if *filter.HasLyrics {
			clauses = append(clauses, "(lyrics != '' OR synced_lyrics != '')")
		} else {
			clauses = append(clauses, "(lyrics = '' AND synced_lyrics = '')")
		}
	}
	if filter.HasCoverArt != nil {
		if *filter.HasCoverArt {
			clauses = append(clauses, "cover_art_path != ''")
		} else {
			clauses = append(clauses, "cover_art_path = ''")
		}
	}
	if filter.Search != "" {
		clauses = append(clauses, "(path LIKE '%'||?||'%' COLLATE NOCASE OR artist LIKE '%'||?||'%' COLLATE NOCASE OR album LIKE '%'||?||'%' COLLATE NOCASE OR title LIKE '%'||?||'%' COLLATE NOCASE OR raw_title LIKE '%'||?||'%' COLLATE NOCASE OR raw_artist LIKE '%'||?||'%' COLLATE NOCASE OR raw_album LIKE '%'||?||'%' COLLATE NOCASE)")
		args = append(args, filter.Search, filter.Search, filter.Search, filter.Search, filter.Search, filter.Search, filter.Search)
	}

	if len(clauses) == 0 {
		return "", nil
	}
	where := clauses[0]
	for _, c := range clauses[1:] {
		where += " AND " + c
	}
	return where, args
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// QueryPage returns one page of tracked records matching filter, sorted per
// sort with an id-based tie-break, alongside the total matching count.
func (s *SQLiteStore) QueryPage(ctx context.Context, filter usecases.LibraryFilter, sort usecases.LibrarySort, limit, offset int) ([]domain.FileRecord, int, error) {
	where, args := buildLibraryWhere(filter)

	countQuery := "SELECT COUNT(*) FROM files"
	if where != "" {
		countQuery += " WHERE " + where
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting library rows: %w", err)
	}

	column, ok := librarySortColumns[sort.By]
	if !ok {
		column = "path"
	}
	direction := "ASC"
	if sort.Desc {
		direction = "DESC"
	}

	query := `
		SELECT path, format, fingerprint, duration_seconds, size, mtime, status, missing, fingerprint_error,
		       artist, album, title, track_number, recording_mbid,
		       album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid,
		       cover_art_path, lyrics, synced_lyrics, tagged, tag_error, relocated, relocate_error,
		       raw_title, raw_artist, raw_album, raw_album_artist
		FROM files`
	if where != "" {
		query += " WHERE " + where
	}
	query += fmt.Sprintf(" ORDER BY %s %s, id ASC LIMIT ? OFFSET ?", column, direction)

	pageArgs := append(append([]any{}, args...), limit, offset)
	rows, err := s.db.QueryContext(ctx, query, pageArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying library page: %w", err)
	}
	defer rows.Close()

	var entries []domain.FileRecord
	for rows.Next() {
		var rec domain.FileRecord
		var format, status string
		var missing, tagged, relocated int
		if err := rows.Scan(&rec.Path, &format, &rec.Fingerprint, &rec.DurationSeconds, &rec.Size, &rec.ModTime, &status, &missing, &rec.FingerprintError,
			&rec.Artist, &rec.Album, &rec.Title, &rec.TrackNumber, &rec.RecordingMBID,
			&rec.AlbumArtist, &rec.Year, &rec.DiscNumber, &rec.TotalDiscs, &rec.TotalTracks, &rec.ReleaseMBID, &rec.ReleaseGroupMBID, &rec.ArtistMBID,
			&rec.CoverArtPath, &rec.Lyrics, &rec.SyncedLyrics, &tagged, &rec.TagError, &relocated, &rec.RelocateError,
			&rec.RawTitle, &rec.RawArtist, &rec.RawAlbum, &rec.RawAlbumArtist); err != nil {
			return nil, 0, fmt.Errorf("scanning library page row: %w", err)
		}
		rec.Format = domain.Format(format)
		rec.Status = domain.TrackingStatus(status)
		rec.Missing = missing != 0
		rec.Tagged = tagged != 0
		rec.Relocated = relocated != 0
		entries = append(entries, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("reading library page rows: %w", err)
	}

	return entries, total, nil
}

// QueryPaths returns every path matching filter, ignoring pagination — used
// to resolve a bulk action's filter-based selection at execution time.
func (s *SQLiteStore) QueryPaths(ctx context.Context, filter usecases.LibraryFilter) ([]string, error) {
	where, args := buildLibraryWhere(filter)

	query := "SELECT path FROM files"
	if where != "" {
		query += " WHERE " + where
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying matching paths: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("scanning matching path: %w", err)
		}
		paths = append(paths, path)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading matching paths: %w", err)
	}

	return paths, nil
}

// Delete removes one tracked record entirely. A plain, ungated row delete —
// callers decide when deletion is allowed.
func (s *SQLiteStore) Delete(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM files WHERE path = ?`, path)
	if err != nil {
		return fmt.Errorf("deleting tracked record for %s: %w", path, err)
	}
	return nil
}

// PathsUnder returns every tracked record whose path starts with prefix,
// unfiltered and unpaginated — TreeBrowse groups the result into
// subdirectories vs. direct files in memory.
func (s *SQLiteStore) PathsUnder(ctx context.Context, prefix string) ([]domain.FileRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT path, format, fingerprint, duration_seconds, size, mtime, status, missing, fingerprint_error,
		       artist, album, title, track_number, recording_mbid,
		       album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid,
		       cover_art_path, lyrics, synced_lyrics, tagged, tag_error, relocated, relocate_error,
		       raw_title, raw_artist, raw_album, raw_album_artist
		FROM files
		WHERE path LIKE ?
	`, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("querying paths under %s: %w", prefix, err)
	}
	defer rows.Close()

	var records []domain.FileRecord
	for rows.Next() {
		var rec domain.FileRecord
		var format, status string
		var missing, tagged, relocated int
		if err := rows.Scan(&rec.Path, &format, &rec.Fingerprint, &rec.DurationSeconds, &rec.Size, &rec.ModTime, &status, &missing, &rec.FingerprintError,
			&rec.Artist, &rec.Album, &rec.Title, &rec.TrackNumber, &rec.RecordingMBID,
			&rec.AlbumArtist, &rec.Year, &rec.DiscNumber, &rec.TotalDiscs, &rec.TotalTracks, &rec.ReleaseMBID, &rec.ReleaseGroupMBID, &rec.ArtistMBID,
			&rec.CoverArtPath, &rec.Lyrics, &rec.SyncedLyrics, &tagged, &rec.TagError, &relocated, &rec.RelocateError,
			&rec.RawTitle, &rec.RawArtist, &rec.RawAlbum, &rec.RawAlbumArtist); err != nil {
			return nil, fmt.Errorf("scanning path-under row: %w", err)
		}
		rec.Format = domain.Format(format)
		rec.Status = domain.TrackingStatus(status)
		rec.Missing = missing != 0
		rec.Tagged = tagged != 0
		rec.Relocated = relocated != 0
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading path-under rows: %w", err)
	}

	return records, nil
}

// artistKeyPredicate translates an artist grouping key (as produced by
// usecases.GroupArtists / returned in ArtistSummary.Key) into a
// parameterized WHERE clause fragment matching exactly the files belonging
// to that grouping: an ArtistMBID equality for an MBID-keyed group, or an
// unidentified (blank artist_mbid) name match for a "name:"-prefixed key.
func artistKeyPredicate(artistKey string) (string, []any) {
	if strings.HasPrefix(artistKey, "name:") {
		name := strings.TrimPrefix(artistKey, "name:")
		return "(artist_mbid = '' AND COALESCE(NULLIF(artist, ''), NULLIF(raw_artist, ''), ?) = ?)",
			[]any{usecases.UnknownArtist, name}
	}
	return "artist_mbid = ?", []any{artistKey}
}

// albumKeyPredicate is artistKeyPredicate's album-grouping counterpart,
// scoped by its caller alongside an artistKeyPredicate clause.
func albumKeyPredicate(albumKey string) (string, []any) {
	if strings.HasPrefix(albumKey, "name:") {
		name := strings.TrimPrefix(albumKey, "name:")
		return "(release_group_mbid = '' AND COALESCE(NULLIF(album, ''), NULLIF(raw_album, ''), ?) = ?)",
			[]any{usecases.UnknownAlbum, name}
	}
	return "release_group_mbid = ?", []any{albumKey}
}

// ListArtists returns every distinct artist grouping honoring filter,
// computed in Go by usecases.GroupArtists from the raw per-file rows
// matching filter — see that function for the MBID-first grouping and
// mismatch-detection rules.
func (s *SQLiteStore) ListArtists(ctx context.Context, filter usecases.LibraryFilter) ([]usecases.ArtistSummary, error) {
	where, args := buildLibraryWhere(filter)

	query := `SELECT artist, raw_artist, artist_mbid FROM files`
	if where != "" {
		query += " WHERE " + where
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying artists: %w", err)
	}
	defer rows.Close()

	var artistRows []usecases.ArtistRow
	for rows.Next() {
		var r usecases.ArtistRow
		if err := rows.Scan(&r.Artist, &r.RawArtist, &r.ArtistMBID); err != nil {
			return nil, fmt.Errorf("scanning artist row: %w", err)
		}
		artistRows = append(artistRows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading artist rows: %w", err)
	}

	return usecases.GroupArtists(artistRows), nil
}

// ListAlbums returns every distinct album grouping for the artist grouping
// identified by artistKey honoring filter, computed in Go by
// usecases.GroupAlbums from the raw per-file rows matching both the artist
// key and filter.
func (s *SQLiteStore) ListAlbums(ctx context.Context, artistKey string, filter usecases.LibraryFilter) ([]usecases.AlbumSummary, error) {
	where, args := buildLibraryWhere(filter)
	artistClause, artistArgs := artistKeyPredicate(artistKey)

	query := `SELECT album, raw_album, release_group_mbid FROM files WHERE ` + artistClause
	queryArgs := append([]any{}, artistArgs...)
	if where != "" {
		query += " AND " + where
		queryArgs = append(queryArgs, args...)
	}

	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying albums for %s: %w", artistKey, err)
	}
	defer rows.Close()

	var albumRows []usecases.AlbumRow
	for rows.Next() {
		var r usecases.AlbumRow
		if err := rows.Scan(&r.Album, &r.RawAlbum, &r.ReleaseGroupMBID); err != nil {
			return nil, fmt.Errorf("scanning album row: %w", err)
		}
		albumRows = append(albumRows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading album rows: %w", err)
	}

	return usecases.GroupAlbums(albumRows), nil
}

// ResolveArtistKey resolves an artist display name to its grouping key: the
// first non-blank ArtistMBID among files whose resolved-or-raw-or-unknown
// name matches, or a "name:"-prefixed key if none is identified. See
// usecases.TrackingStore.ResolveArtistKey for the label-collision caveat.
func (s *SQLiteStore) ResolveArtistKey(ctx context.Context, name string) (string, error) {
	if name == "" {
		return "", nil
	}
	var mbid string
	row := s.db.QueryRowContext(ctx, `
		SELECT artist_mbid FROM files
		WHERE COALESCE(NULLIF(artist, ''), NULLIF(raw_artist, ''), ?) = ?
		  AND artist_mbid != ''
		LIMIT 1`, usecases.UnknownArtist, name)
	switch err := row.Scan(&mbid); {
	case err == nil:
		return mbid, nil
	case errors.Is(err, sql.ErrNoRows):
		return "name:" + name, nil
	default:
		return "", fmt.Errorf("resolving artist key for %s: %w", name, err)
	}
}

// ResolveAlbumKey resolves an album display name, scoped to the artist
// grouping identified by artistKey, to its grouping key — the album-level
// counterpart to ResolveArtistKey.
func (s *SQLiteStore) ResolveAlbumKey(ctx context.Context, artistKey, albumName string) (string, error) {
	if albumName == "" {
		return "", nil
	}
	artistClause, artistArgs := artistKeyPredicate(artistKey)

	query := `
		SELECT release_group_mbid FROM files
		WHERE ` + artistClause + ` AND COALESCE(NULLIF(album, ''), NULLIF(raw_album, ''), ?) = ?
		  AND release_group_mbid != ''
		LIMIT 1`
	args := append(append([]any{}, artistArgs...), usecases.UnknownAlbum, albumName)

	var mbid string
	row := s.db.QueryRowContext(ctx, query, args...)
	switch err := row.Scan(&mbid); {
	case err == nil:
		return mbid, nil
	case errors.Is(err, sql.ErrNoRows):
		return "name:" + albumName, nil
	default:
		return "", fmt.Errorf("resolving album key for %s: %w", albumName, err)
	}
}

// ListTracks returns the tracks belonging to the artist/album groupings
// identified by artistKey/albumKey honoring filter, sorted by track number.
func (s *SQLiteStore) ListTracks(ctx context.Context, artistKey, albumKey string, filter usecases.LibraryFilter) ([]domain.FileRecord, error) {
	where, args := buildLibraryWhere(filter)
	artistClause, artistArgs := artistKeyPredicate(artistKey)
	albumClause, albumArgs := albumKeyPredicate(albumKey)

	query := `
		SELECT path, format, fingerprint, duration_seconds, size, mtime, status, missing, fingerprint_error,
		       artist, album, title, track_number, recording_mbid,
		       album_artist, year, disc_number, total_discs, total_tracks, release_mbid, release_group_mbid, artist_mbid,
		       cover_art_path, lyrics, synced_lyrics, tagged, tag_error, relocated, relocate_error,
		       raw_title, raw_artist, raw_album, raw_album_artist
		FROM files
		WHERE ` + artistClause + ` AND ` + albumClause
	queryArgs := append(append([]any{}, artistArgs...), albumArgs...)
	if where != "" {
		query += " AND " + where
		queryArgs = append(queryArgs, args...)
	}
	query += " ORDER BY track_number ASC, path ASC"

	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("querying tracks for %s/%s: %w", artistKey, albumKey, err)
	}
	defer rows.Close()

	var records []domain.FileRecord
	for rows.Next() {
		var rec domain.FileRecord
		var format, status string
		var missing, tagged, relocated int
		if err := rows.Scan(&rec.Path, &format, &rec.Fingerprint, &rec.DurationSeconds, &rec.Size, &rec.ModTime, &status, &missing, &rec.FingerprintError,
			&rec.Artist, &rec.Album, &rec.Title, &rec.TrackNumber, &rec.RecordingMBID,
			&rec.AlbumArtist, &rec.Year, &rec.DiscNumber, &rec.TotalDiscs, &rec.TotalTracks, &rec.ReleaseMBID, &rec.ReleaseGroupMBID, &rec.ArtistMBID,
			&rec.CoverArtPath, &rec.Lyrics, &rec.SyncedLyrics, &tagged, &rec.TagError, &relocated, &rec.RelocateError,
			&rec.RawTitle, &rec.RawArtist, &rec.RawAlbum, &rec.RawAlbumArtist); err != nil {
			return nil, fmt.Errorf("scanning track row: %w", err)
		}
		rec.Format = domain.Format(format)
		rec.Status = domain.TrackingStatus(status)
		rec.Missing = missing != 0
		rec.Tagged = tagged != 0
		rec.Relocated = relocated != 0
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading track rows: %w", err)
	}

	return records, nil
}
