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
	Status           TrackingStatus // one of StatusNew, StatusIdentified, StatusNotFound
	Missing          bool
	FingerprintError string // non-empty when the most recent fingerprint attempt failed

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
}

// EffectiveStatus returns the externally-visible status: StatusMissing when
// the file is currently absent from disk, otherwise the real Status.
func (f FileRecord) EffectiveStatus() TrackingStatus {
	if f.Missing {
		return StatusMissing
	}
	return f.Status
}
