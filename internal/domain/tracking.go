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
}

// EffectiveStatus returns the externally-visible status: StatusMissing when
// the file is currently absent from disk, otherwise the real Status.
func (f FileRecord) EffectiveStatus() TrackingStatus {
	if f.Missing {
		return StatusMissing
	}
	return f.Status
}
