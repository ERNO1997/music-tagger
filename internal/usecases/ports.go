package usecases

import (
	"context"

	"music-tagger/internal/domain"
)

// Fingerprinter computes the acoustic identity of a single audio file.
// Implementations must never derive identity from the file's name or
// any pre-existing embedded tags.
type Fingerprinter interface {
	Fingerprint(ctx context.Context, path string) (domain.Fingerprint, error)
}

// TrackingStore persists per-file discovery/identification state across
// refreshes and restarts.
type TrackingStore interface {
	// LoadAll returns every tracked record, keyed by path, for in-memory
	// diffing against a fresh disk walk.
	LoadAll(ctx context.Context) (map[string]domain.FileRecord, error)

	// BulkApply commits the outcome of one refresh pass in a single
	// transaction: new/changed file upserts, paths to mark missing, and
	// previously-missing paths that reappeared unchanged.
	BulkApply(ctx context.Context, apply BulkApply) error

	// RecordIdentification updates one file's status and (when identified)
	// resolved metadata in a single commit, without altering its
	// fingerprint, size, or modification time.
	RecordIdentification(ctx context.Context, path string, result IdentificationResult) error

	// RecordCoverArt updates one file's stored cover art path, without
	// altering its fingerprint, status, or resolved metadata.
	RecordCoverArt(ctx context.Context, path string, coverArtPath string) error

	// GetCoverArtPath returns one file's stored cover art path (a single
	// indexed lookup, not a full LoadAll) — used to serve cover images,
	// which would otherwise mean one full-table load per rendered
	// thumbnail.
	GetCoverArtPath(ctx context.Context, path string) (coverArtPath string, found bool, err error)

	// RecordLyrics updates one file's stored plain and synced lyrics,
	// without altering its fingerprint, status, or resolved metadata.
	RecordLyrics(ctx context.Context, path string, lyrics string, syncedLyrics string) error

	// GetLyrics returns one file's stored plain and synced lyrics (a
	// single indexed lookup, not a full LoadAll) — used to serve lyrics
	// on demand from the details view.
	GetLyrics(ctx context.Context, path string) (lyrics string, syncedLyrics string, found bool, err error)

	// Get returns one file's complete tracked record (a single indexed
	// lookup, not a full LoadAll) — used by tagging to load one file's
	// resolved metadata/cover art path/lyrics without loading the whole
	// table.
	Get(ctx context.Context, path string) (record domain.FileRecord, found bool, err error)

	// RecordTagged updates one file's tagged outcome, without altering its
	// fingerprint, status, resolved metadata, cover art path, or lyrics.
	// tagErr is empty on a successful tag write.
	RecordTagged(ctx context.Context, path string, tagged bool, tagErr string) error

	// RecordFileStat updates one file's stored size and modification time,
	// without altering any other field. Used after a successful tag write:
	// writing tags changes the file's own size/mtime on disk, and without
	// this, the next scan would compare its stale pre-tag baseline against
	// the file's real post-tag stat, conclude the file "changed", and reset
	// its status and resolved metadata to blank.
	RecordFileStat(ctx context.Context, path string, size int64, modTime int64) error

	// RecordRelocation updates one file's path to its new, post-relocation
	// location and marks it relocated, in a single commit, without
	// altering any other field.
	RecordRelocation(ctx context.Context, oldPath, newPath string) error

	// RecordRelocationFailure updates one file's relocation outcome to
	// failed with the given reason, without altering its path or any
	// other field.
	RecordRelocationFailure(ctx context.Context, path string, relocateErr string) error

	// QueryPage returns one page of tracked records matching filter, sorted
	// per sort with a stable tie-break, alongside the total count of
	// matching records independent of limit/offset. Distinct from LoadAll,
	// which is unfiltered and unpaginated and used only for scan's internal
	// change-detection diffing.
	QueryPage(ctx context.Context, filter LibraryFilter, sort LibrarySort, limit, offset int) (entries []domain.FileRecord, total int, err error)

	// QueryPaths returns every path matching filter, ignoring pagination —
	// used to resolve a bulk action's filter-based selection into a
	// concrete path list at the moment it executes.
	QueryPaths(ctx context.Context, filter LibraryFilter) ([]string, error)

	// Delete removes one tracked record entirely. A plain, ungated row
	// delete — callers are responsible for deciding when deletion is
	// allowed (see the DeleteMissingFile usecase).
	Delete(ctx context.Context, path string) error
}

// LibraryFilter narrows a QueryPage/QueryPaths read. A zero-value
// LibraryFilter matches every tracked record.
type LibraryFilter struct {
	// Status is "" (no filter) or a domain.TrackingStatus value, applied
	// against each record's EffectiveStatus rather than its stored Status —
	// filtering by StatusMissing means Missing is set; filtering by any
	// other status means Missing is clear and Status matches.
	Status string

	// Tagged and Relocated are nil (no filter) or a pointer to the exact
	// boolean value each matching record's field must equal.
	Tagged    *bool
	Relocated *bool

	// Search is a case-insensitive substring match against path, artist,
	// album, and title. Empty means no filter.
	Search string
}

// LibrarySort orders a QueryPage read. By must be one of the allow-listed
// sort keys (path, status, artist, album, duration, year); an unrecognized
// or empty value falls back to path ascending.
type LibrarySort struct {
	By   string
	Desc bool
}

// BulkApply is the batched result of one refresh pass.
type BulkApply struct {
	// Upserts are new or changed files: inserted or updated with a fresh
	// fingerprint, status reset to StatusNew, Missing cleared.
	Upserts []domain.FileRecord

	// MissingPaths are previously tracked paths not found on this pass;
	// their Missing flag is set without altering any other field.
	MissingPaths []string

	// ReappearedPaths are paths previously marked missing that were found
	// again unchanged; their Missing flag is cleared, restoring their
	// prior Status without altering it.
	ReappearedPaths []string
}

// AcoustIDMatch is one candidate MusicBrainz recording resolved from a
// fingerprint, ranked by match confidence.
type AcoustIDMatch struct {
	RecordingID string
	Score       float64
}

// AcoustIDLookup resolves a fingerprint + duration to candidate MusicBrainz
// Recording IDs. An empty, nil-error result means AcoustID found no match —
// distinct from a returned error, which means the lookup itself failed.
type AcoustIDLookup interface {
	Lookup(ctx context.Context, fingerprint string, durationSeconds float64) ([]AcoustIDMatch, error)
}

// RecordingMetadata is the canonical metadata MusicBrainz resolves for a
// given Recording ID.
type RecordingMetadata struct {
	RecordingID string
	Artist      string
	Album       string
	Title       string
	TrackNumber int

	// Extended fields, all derived from the same recording lookup — no
	// additional MusicBrainz request. Year/DiscNumber/TotalDiscs/
	// TotalTracks are 0 when not derivable (e.g. no usable release date).
	AlbumArtist      string
	Year             int
	DiscNumber       int
	TotalDiscs       int
	TotalTracks      int
	ReleaseMBID      string
	ReleaseGroupMBID string
	ArtistMBID       string
}

// MusicBrainzLookup resolves a MusicBrainz Recording ID to canonical
// artist/release/track data. Implementations must enforce the 1 req/sec
// rate limit centrally, regardless of caller.
type MusicBrainzLookup interface {
	Lookup(ctx context.Context, recordingID string) (RecordingMetadata, error)
}

// IdentificationResult is the outcome of attempting to identify one file.
type IdentificationResult struct {
	Status   domain.TrackingStatus // StatusIdentified or StatusNotFound
	Metadata RecordingMetadata     // populated only when Status is StatusIdentified
}

// CoverArtLookup resolves a MusicBrainz Release ID to front-cover image
// bytes via Cover Art Archive, falling back to the Release-Group ID if the
// specific release has no art (a release-group can have many sibling
// editions, and not all of them have art uploaded). A nil byte slice with
// a nil error means no cover art is available anywhere in the
// release-group — distinct from a returned error, which means the lookup
// itself failed.
type CoverArtLookup interface {
	Lookup(ctx context.Context, releaseMBID, releaseGroupMBID string) ([]byte, error)
}

// LyricsLookup resolves an already-known artist/title/album/duration to
// plain and, when available, LRC-timed synced lyrics via LRCLIB. found=false
// with a nil error means no lyrics are available (not found, or the track
// is instrumental) — distinct from a returned error, which means the lookup
// itself failed.
type LyricsLookup interface {
	Lookup(ctx context.Context, artist, title, album string, durationSeconds int) (plainLyrics, syncedLyrics string, found bool, err error)
}

// TagInput is one already-identified tracked file's resolved metadata,
// cover art, and lyrics, in the shape needed to write it into the physical
// file's own tags.
type TagInput struct {
	Artist      string
	Album       string
	Title       string
	AlbumArtist string
	TrackNumber int
	TotalTracks int
	DiscNumber  int
	TotalDiscs  int
	Year        int

	// CoverArt is the image bytes to embed, or nil if no cover art is
	// stored for this file.
	CoverArt []byte

	// Lyrics is the plain lyrics text to embed, or empty if none is
	// stored for this file.
	Lyrics string
}

// EmbeddedTags is what's actually, currently embedded in a physical audio
// file's own tags, read live from disk — independent of (and not to be
// confused with) the resolved metadata cached in the tracking store.
type EmbeddedTags struct {
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	TrackNumber int
	DiscNumber  int
	Year        int

	HasLyrics   bool
	HasCoverArt bool
}

// Tagger writes resolved metadata, cover art, and lyrics into an audio
// file's own tag format (ID3v2 for MP3, Vorbis comments for FLAC, MP4
// atoms for M4A) at its current path, and can read a file's actual
// currently-embedded tags back for verification. Implementations must
// preserve any existing tag data not covered by TagInput/EmbeddedTags.
type Tagger interface {
	Tag(ctx context.Context, path string, meta TagInput) error
	ReadEmbeddedTags(ctx context.Context, path string) (EmbeddedTags, error)
}

// RelocateInput is the resolved metadata needed to compute an
// already-identified-and-tagged file's canonical destination path.
type RelocateInput struct {
	Artist      string
	Album       string
	Title       string
	TrackNumber int

	// Year prefixes the album directory name ("{Year} - {Album}") when
	// positive. 0 means the release had no usable date (see
	// RecordingMetadata.Year) — the album directory is then just the
	// album name, with no prefix.
	Year int
}

// Relocator physically moves an audio file into the canonical
// Artist/Album/Track hierarchy, sanitizing path segments before any
// filesystem call. Implementations must leave the source file untouched
// on any error.
type Relocator interface {
	// Relocate moves the file at path to its computed destination and
	// returns the new path. path is left untouched if an error is
	// returned (including a destination collision).
	Relocate(ctx context.Context, path string, meta RelocateInput) (newPath string, err error)

	// Undo moves a file from currentPath back to originalPath — a bare
	// move with no sanitization or directory creation, used as a
	// best-effort rollback when recording a successful relocation fails.
	Undo(ctx context.Context, currentPath, originalPath string) error
}
