package domain

// TrackingStatus is the identification lifecycle state of a tracked file.
// Missing is derived (see FileRecord.EffectiveStatus), not stored directly
// as Status, so that a file's real identification state survives being
// temporarily missing and can be restored without loss when it reappears.
type TrackingStatus string

const (
	StatusNew        TrackingStatus = "new"
	StatusIdentified TrackingStatus = "identified"
	StatusNotFound   TrackingStatus = "not_found"
	StatusAmbiguous  TrackingStatus = "ambiguous"
	StatusMissing    TrackingStatus = "missing"
)

// FileRecord is a persisted, per-file tracking row: what was last seen on
// disk for this path, and its identification lifecycle state.
type FileRecord struct {
	Path             string
	Format           Format
	Fingerprint      string
	DurationSeconds  float64
	Size             int64
	ModTime          int64          // Unix seconds
	Status           TrackingStatus // one of StatusNew, StatusIdentified, StatusNotFound, StatusAmbiguous
	Missing          bool
	FingerprintError string // non-empty when the most recent fingerprint attempt failed

	// RawTitle/RawArtist/RawAlbum/RawAlbumArtist are a snapshot of the
	// file's own embedded tags, captured during scan — independent of, and
	// not to be confused with, resolved (AcoustID/MusicBrainz) metadata
	// below. Populated for new/changed files when available; blank if the
	// file has no such tags or they couldn't be read. Never written by
	// identification and never used as an identification signal.
	RawTitle       string
	RawArtist      string
	RawAlbum       string
	RawAlbumArtist string

	// Resolved metadata, populated only once Status is StatusIdentified.
	Artist        string
	Album         string
	Title         string
	TrackNumber   int
	RecordingMBID string

	// Extended resolved metadata, also populated only once identified.
	// Year is 0 when the release had no usable date.
	AlbumArtist      string
	Year             int
	DiscNumber       int
	TotalDiscs       int
	TotalTracks      int
	ReleaseMBID      string
	ReleaseGroupMBID string
	ArtistMBID       string

	// CoverArtPath is the on-disk path to this file's downloaded cover
	// art, populated only after enrichment. Empty if not yet enriched or
	// if no cover art was available for the release.
	CoverArtPath string

	// Lyrics and SyncedLyrics are populated only after enrichment. Empty
	// if not yet enriched, or if LRCLIB had no entry or marked the track
	// instrumental. SyncedLyrics may be empty even when Lyrics is set —
	// many LRCLIB entries have only one or the other.
	Lyrics       string
	SyncedLyrics string

	// Tagged and TagError reflect the outcome of the most recent attempt
	// to write resolved metadata/cover art/lyrics into the physical file's
	// own tags. TagError is non-empty only when a tag write was attempted
	// and failed.
	Tagged   bool
	TagError string

	// Relocated and RelocateError reflect the outcome of the most recent
	// attempt to physically move the file into the canonical
	// Artist/Album/Track hierarchy. RelocateError is non-empty only when a
	// relocation was attempted and failed. On success, Path already
	// reflects the file's new location — relocation updates the same
	// record rather than replacing it.
	Relocated     bool
	RelocateError string
}

// EffectiveStatus returns the externally-visible status: StatusMissing when
// the file is currently absent from disk, otherwise the real Status.
func (f FileRecord) EffectiveStatus() TrackingStatus {
	if f.Missing {
		return StatusMissing
	}
	return f.Status
}
