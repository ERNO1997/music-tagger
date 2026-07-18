package domain

import "time"

// Fingerprint is the Chromaprint acoustic identity of an audio file, as
// computed by fpcalc. It is the only trustworthy basis for identifying a
// track — filenames and embedded tags are never used.
type Fingerprint struct {
	Chroma   string
	Duration time.Duration
}
