package filestat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"music-tagger/internal/domain"
)

// FpcalcRunner is a Fingerprinter that orchestrates the system `fpcalc`
// binary via os/exec. It never inspects the file's name or embedded tags —
// only fpcalc's decoded-audio output determines the fingerprint.
type FpcalcRunner struct {
	// BinaryPath overrides the fpcalc executable name/path. Defaults to
	// "fpcalc" (resolved via PATH) when empty.
	BinaryPath string
}

func NewFpcalcRunner() *FpcalcRunner {
	return &FpcalcRunner{BinaryPath: "fpcalc"}
}

type fpcalcOutput struct {
	Duration    float64 `json:"duration"`
	Fingerprint string  `json:"fingerprint"`
}

func (r *FpcalcRunner) Fingerprint(ctx context.Context, path string) (domain.Fingerprint, error) {
	if !isSupportedExtension(path) {
		return domain.Fingerprint{}, fmt.Errorf("%w: %s", domain.ErrUnsupportedFormat, path)
	}

	bin := r.BinaryPath
	if bin == "" {
		bin = "fpcalc"
	}
	if _, err := exec.LookPath(bin); err != nil {
		return domain.Fingerprint{}, fmt.Errorf("%w: %s", domain.ErrFingerprinterUnavailable, err)
	}

	cmd := exec.CommandContext(ctx, bin, "-json", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// fpcalc can exit non-zero (e.g. a benign decode-EOF warning near the
	// end of the stream) while still having printed a usable fingerprint
	// to stdout. The exit code alone is therefore not a reliable success
	// signal — treat a parseable, non-empty fingerprint as success
	// regardless of exit status, and only surface an error when stdout
	// yields no usable fingerprint.
	runErr := cmd.Run()

	var out fpcalcOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil || out.Fingerprint == "" {
		detail := strings.TrimSpace(stderr.String())
		if runErr != nil {
			return domain.Fingerprint{}, fmt.Errorf("%w: %s: %s", domain.ErrFingerprintFailed, runErr, detail)
		}
		return domain.Fingerprint{}, fmt.Errorf("%w: no fingerprint in output: %s", domain.ErrFingerprintFailed, detail)
	}

	return domain.Fingerprint{
		Chroma:   out.Fingerprint,
		Duration: time.Duration(out.Duration * float64(time.Second)),
	}, nil
}

func isSupportedExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3", ".flac":
		return true
	default:
		return false
	}
}
