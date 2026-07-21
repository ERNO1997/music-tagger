package filestat

import (
	"context"
	"fmt"
	"time"

	taglib "go.senan.xyz/taglib"
)

// TagLibDurationReader is a DurationReader backed by TagLib's audio
// properties read — sourced from the file's own container headers (MP3
// Xing/VBRI frame, FLAC STREAMINFO, MP4 moov atom), not a full audio decode
// like Fingerprinter.
type TagLibDurationReader struct{}

func NewTagLibDurationReader() *TagLibDurationReader {
	return &TagLibDurationReader{}
}

// ReadDuration reads path's duration. Like TagLibTagger, it reads against
// path's real, content-sniffed format rather than trusting a possibly
// mismatched extension.
func (r *TagLibDurationReader) ReadDuration(ctx context.Context, path string) (time.Duration, error) {
	var duration time.Duration
	err := withCorrectExtension(path, func(workingPath string) error {
		props, err := taglib.ReadProperties(workingPath)
		if err != nil {
			return fmt.Errorf("reading properties for %s: %w", path, err)
		}
		duration = props.Length
		return nil
	})
	if err != nil {
		return 0, err
	}
	return duration, nil
}
