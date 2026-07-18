package domain

import "time"

// Format identifies a supported audio container/codec.
type Format string

const (
	FormatMP3  Format = "mp3"
	FormatFLAC Format = "flac"
)

// AudioFile is a physical audio file discovered on disk, identified solely
// by its acoustic Fingerprint rather than its Path.
type AudioFile struct {
	Path       string
	Format     Format
	Duration   time.Duration
	Fingerprint Fingerprint
}
