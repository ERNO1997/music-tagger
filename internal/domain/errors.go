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
)
