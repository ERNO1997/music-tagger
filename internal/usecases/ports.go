package usecases

import (
	"context"
	"time"

	"music-tagger/internal/domain"
)

// Fingerprinter computes the acoustic identity of a single audio file.
// Implementations must never derive identity from the file's name or
// any pre-existing embedded tags.
type Fingerprinter interface {
	Fingerprint(ctx context.Context, path string) (domain.Fingerprint, error)
}

// DurationReader reads a single audio file's duration from its own
// container headers — a cheap read, unlike Fingerprinter, which requires a
// full audio decode.
type DurationReader interface {
	ReadDuration(ctx context.Context, path string) (time.Duration, error)
}

// RawTags is a snapshot of a file's own embedded tags, read directly from
// disk — independent of resolved (AcoustID/MusicBrainz) metadata. Never
// used as an identification signal (see the Acoustic-First Identification
// Rule); purely for display and search before or absent identification.
type RawTags struct {
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
}

// RawTagReader reads a single audio file's own embedded title/artist/
// album/album-artist tags — a cheap, local, no-decode read, same cost
// class as DurationReader.
type RawTagReader interface {
	ReadRawTags(ctx context.Context, path string) (RawTags, error)
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

	// RecordFingerprint updates one file's fingerprint, duration, and
	// fingerprint error, without altering its status, resolved metadata, or
	// any other field. Called once by IdentifyFile.Identify after it
	// computes (or fails to compute) a fingerprint on demand.
	RecordFingerprint(ctx context.Context, path string, fingerprint string, durationSeconds float64, fingerprintErr string) error

	// RecordAmbiguous replaces any existing stored candidates for path with
	// candidates, sets its status to StatusAmbiguous, and clears its
	// resolved metadata and enrichment/tagged/relocated outcomes (mirroring
	// RecordIdentification's not-found invalidation), in one commit.
	RecordAmbiguous(ctx context.Context, path string, candidates []RecordingMetadata) error

	// GetCandidates returns one file's stored candidate list (populated only
	// while its status is StatusAmbiguous) — a single indexed lookup, not a
	// full LoadAll, used to serve the details view's candidate picker.
	GetCandidates(ctx context.Context, path string) ([]RecordingMetadata, error)

	// ResolveAmbiguous records the stored candidate matching recordingMBID as
	// path's resolved identification (exactly as RecordIdentification would
	// for a direct success) and discards its other stored candidates, in one
	// commit. found is false (with a nil error) when recordingMBID doesn't
	// match any of path's stored candidates — nothing is changed in that case.
	ResolveAmbiguous(ctx context.Context, path, recordingMBID string) (found bool, err error)

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

	// PathsUnder returns every tracked record whose path starts with
	// prefix (a plain LIKE 'prefix%' match), unfiltered and unpaginated —
	// used by TreeBrowse to fetch everything under a folder-tree node in
	// one query before grouping it in memory into subdirectories vs.
	// direct files.
	PathsUnder(ctx context.Context, prefix string) ([]domain.FileRecord, error)

	// ListArtists returns every distinct artist grouping honoring filter,
	// keyed by ArtistMBID when present (falling back to a name-derived key
	// for unidentified files — see GroupArtists), each with its
	// representative display name, total matching track count, and
	// mismatch flags.
	ListArtists(ctx context.Context, filter LibraryFilter) ([]ArtistSummary, error)

	// ListAlbums returns every distinct album grouping for the artist
	// grouping identified by artistKey (as returned in ArtistSummary.Key)
	// honoring filter, keyed by ReleaseGroupMBID when present (see
	// GroupAlbums), each with its representative display name, matching
	// track count, and mismatch flags.
	ListAlbums(ctx context.Context, artistKey string, filter LibraryFilter) ([]AlbumSummary, error)

	// ListTracks returns the tracks belonging to the artist/album groupings
	// identified by artistKey/albumKey honoring filter, sorted by track
	// number.
	ListTracks(ctx context.Context, artistKey, albumKey string, filter LibraryFilter) ([]domain.FileRecord, error)

	// ResolveArtistKey resolves an artist display name to its current
	// grouping key, for backward compatibility with callers that identify
	// an artist by name rather than by the key ListArtists returns. Lossy
	// exactly when a label collision exists (see ArtistSummary.LabelCollision)
	// — a name alone can't disambiguate two groups sharing a display label.
	ResolveArtistKey(ctx context.Context, name string) (string, error)

	// ResolveAlbumKey resolves an album display name, scoped to the artist
	// grouping identified by artistKey, to its current grouping key — the
	// album-level counterpart to ResolveArtistKey, with the same
	// label-collision caveat.
	ResolveAlbumKey(ctx context.Context, artistKey, albumName string) (string, error)
}

// UnknownArtist and UnknownAlbum are the distinguished bucket names
// ListArtists/ListAlbums/ListTracks group a tracked file under when it has
// neither resolved metadata nor a raw tag snapshot for that dimension.
const (
	UnknownArtist = "(Unknown Artist)"
	UnknownAlbum  = "(Unknown Album)"
)

// ArtistSummary is one distinct artist grouping, as returned by
// ListArtists. Key is the grouping key (ArtistMBID when present, else a
// name-derived key) used to unambiguously select this grouping in
// subsequent ListAlbums/ListTracks/completeness-check calls — necessary
// because Artist (the display label) can collide across two different
// groupings (see LabelCollision).
type ArtistSummary struct {
	Key        string
	Artist     string
	TrackCount int

	// NameMismatch is true when this grouping's files share one MBID but
	// disagree on the resolved/raw artist name string; DistinctNames then
	// holds the names observed. Always false for a name-derived key (there
	// is no MBID to disagree about).
	NameMismatch bool

	// LabelCollision is true when another grouping (a different Key)
	// resolves to the same display label as this one.
	LabelCollision bool

	// DistinctNames holds every distinct name observed in this grouping,
	// populated only when NameMismatch is true.
	DistinctNames []string
}

// AlbumSummary is one distinct album grouping for a given artist grouping,
// as returned by ListAlbums. See ArtistSummary for the meaning of Key,
// NameMismatch, LabelCollision, and DistinctNames — identical rules, scoped
// to release-group MBID and album name instead of artist MBID and name.
type AlbumSummary struct {
	Key            string
	Album          string
	TrackCount     int
	NameMismatch   bool
	LabelCollision bool
	DistinctNames  []string
}

// LibraryFilter narrows a QueryPage/QueryPaths read. A zero-value
// LibraryFilter matches every tracked record.
type LibraryFilter struct {
	// Status is "" (no filter) or a domain.TrackingStatus value, applied
	// against each record's EffectiveStatus rather than its stored Status —
	// filtering by StatusMissing means Missing is set; filtering by any
	// other status means Missing is clear and Status matches.
	Status string

	// Tagged, Relocated, HasLyrics, and HasCoverArt are nil (no filter) or a
	// pointer to the exact boolean value each matching record's field must
	// equal. HasLyrics matches a record whose stored plain or synced
	// lyrics are non-empty (true) or both empty (false). HasCoverArt
	// matches a record whose stored cover art path is non-empty (true) or
	// empty (false).
	Tagged      *bool
	Relocated   *bool
	HasLyrics   *bool
	HasCoverArt *bool

	// Search is a case-insensitive substring match against path, artist,
	// album, and title. Empty means no filter.
	Search string

	// Paths restricts a read to exactly these paths when non-empty, taking
	// priority over every other field above (which are ignored entirely in
	// that case) — mirroring how resolveSelection already treats an
	// explicit path list as taking priority over a filter for the trigger
	// endpoints.
	Paths []string
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

// AcoustIDResult is one scored match from AcoustID's fingerprint index.
// RecordingIDs holds every MusicBrainz recording tied to this one result —
// usually exactly one, but a single acoustic fingerprint can map to more
// than one distinct recording (a reissue, a compilation reusing the same
// master, etc.), and callers need to tell that apart from an unambiguous
// single-recording match rather than having it silently flattened away.
type AcoustIDResult struct {
	Score        float64
	RecordingIDs []string
}

// AcoustIDLookup resolves a fingerprint + duration to candidate MusicBrainz
// Recording IDs, grouped by result and ranked by descending score. An
// empty, nil-error result means AcoustID found no match — distinct from a
// returned error, which means the lookup itself failed.
type AcoustIDLookup interface {
	Lookup(ctx context.Context, fingerprint string, durationSeconds float64) ([]AcoustIDResult, error)
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

// MusicBrainzSearch resolves a free-text query directly to candidate
// recordings, independent of any AcoustID fingerprint match — used for
// manual, user-initiated identification when a file's audio can't be
// (or wasn't correctly) fingerprint-matched. An empty, nil-error result
// means the query matched nothing — distinct from a returned error, which
// means the search itself failed.
type MusicBrainzSearch interface {
	Search(ctx context.Context, query string, limit int) ([]RecordingMetadata, error)
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

// ReleaseGroupRelease is one sibling release belonging to a release-group,
// as returned by MusicBrainzReleaseGroupLookup — used to browse alternate
// cover art across a release-group's editions.
type ReleaseGroupRelease struct {
	ReleaseMBID string
	Title       string
	Status      string
	Date        string
}

// MusicBrainzReleaseGroupLookup resolves a release-group's sibling
// releases. Implementations must enforce the same centralized rate limit
// as MusicBrainzLookup, since both hit the same MusicBrainz web service.
type MusicBrainzReleaseGroupLookup interface {
	Releases(ctx context.Context, releaseGroupMBID string) ([]ReleaseGroupRelease, error)
}

// ArtistReleaseGroupSummary is one album in an artist's MusicBrainz
// discography, as returned by MusicBrainzDiscographyLookup.ArtistReleaseGroups.
type ArtistReleaseGroupSummary struct {
	ReleaseGroupMBID string
	Title            string
	Year             int
}

// ReleaseTrackSummary is one track on a MusicBrainz release, as returned by
// MusicBrainzDiscographyLookup.ReleaseTracklist.
type ReleaseTrackSummary struct {
	RecordingMBID string
	Title         string
	TrackNumber   int
}

// MusicBrainzDiscographyLookup resolves an artist's full release-group
// catalog and a release's full tracklist, for comparing the local library
// against MusicBrainz's actual catalog (the "completeness check"). Both
// operations are subject to the same centralized rate limit as
// MusicBrainzLookup, since they hit the same MusicBrainz web service.
type MusicBrainzDiscographyLookup interface {
	ArtistReleaseGroups(ctx context.Context, artistMBID string) ([]ArtistReleaseGroupSummary, error)
	ReleaseTracklist(ctx context.Context, releaseMBID string) ([]ReleaseTrackSummary, error)
}

// CoverArtCandidate is one release's front-cover image, offered as an
// alternative when browsing a release-group's sibling editions for a
// better cover than CoverArtLookup's single automatic choice.
type CoverArtCandidate struct {
	ReleaseMBID  string
	ReleaseTitle string
	ThumbnailURL string // small preview, for a browsing grid
	ImageURL     string // same size class CoverArtLookup embeds, passed back to Download when chosen
}

// CoverArtBrowser lists a release's front-cover image (without downloading
// its bytes) and downloads a specifically chosen image's bytes. A separate
// capability from CoverArtLookup, which only ever returns one
// automatically-chosen image.
type CoverArtBrowser interface {
	// FrontImage returns a release's front-cover thumbnail/image URLs.
	// found=false (nil error) means that release has no front image
	// uploaded — distinct from a returned error, which means the lookup
	// itself failed.
	FrontImage(ctx context.Context, releaseMBID string) (thumbnailURL, imageURL string, found bool, err error)

	// Download fetches the image bytes at imageURL (as returned by
	// FrontImage). Implementations must reject a URL outside Cover Art
	// Archive's own domain, since imageURL round-trips through API/UI
	// input on the Choose path.
	Download(ctx context.Context, imageURL string) ([]byte, error)
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

	// RecordingMBID, ReleaseMBID, ReleaseGroupMBID, and ArtistMBID are
	// written into the file's own tags (e.g. a UFID frame for the
	// recording ID on MP3, TXXX/Vorbis-comment/MP4-freeform-atom for the
	// rest) whenever resolved, so identification survives independently
	// of the tracking store.
	RecordingMBID    string
	ReleaseMBID      string
	ReleaseGroupMBID string
	ArtistMBID       string

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

	RecordingMBID    string
	ReleaseMBID      string
	ReleaseGroupMBID string
	ArtistMBID       string

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

	// ReadEmbeddedContent reads path's actual embedded cover image bytes
	// and lyrics text, live from disk — the same underlying data
	// ReadEmbeddedTags summarizes as HasCoverArt/HasLyrics booleans, used
	// by the background-library-analysis capability to store the content
	// itself rather than just detect its presence. coverArt is nil and
	// lyrics is empty when absent.
	ReadEmbeddedContent(ctx context.Context, path string) (coverArt []byte, lyrics string, err error)
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
