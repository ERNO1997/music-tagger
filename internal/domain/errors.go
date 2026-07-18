package domain

import "errors"

var (
	// ErrUnsupportedFormat is returned when a file's extension is not a
	// supported audio format (.mp3, .flac) and must not be fingerprinted.
	ErrUnsupportedFormat = errors.New("unsupported audio format")

	// ErrFingerprinterUnavailable is returned when the fpcalc executable
	// cannot be found on PATH.
	ErrFingerprinterUnavailable = errors.New("fpcalc executable not found")

	// ErrFingerprintFailed is returned when fpcalc runs but fails to
	// produce a usable fingerprint for a file (non-zero exit or
	// unparseable output).
	ErrFingerprintFailed = errors.New("fingerprint computation failed")

	// ErrAcoustIDNotConfigured is returned when an AcoustID lookup is
	// attempted without an API key configured.
	ErrAcoustIDNotConfigured = errors.New("ACOUSTID_API_KEY not configured")

	// ErrMusicBrainzNotConfigured is returned when a MusicBrainz lookup is
	// attempted without a User-Agent configured.
	ErrMusicBrainzNotConfigured = errors.New("MUSICBRAINZ_USER_AGENT not configured")

	// ErrNoMusicBrainzRelease is returned when a MusicBrainz recording
	// resolves but has no associated releases to derive metadata from.
	ErrNoMusicBrainzRelease = errors.New("no MusicBrainz release found for recording")
)
