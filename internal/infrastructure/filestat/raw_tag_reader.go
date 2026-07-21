package filestat

import (
	"context"
	"fmt"

	taglib "go.senan.xyz/taglib"

	"music-tagger/internal/usecases"
)

// TagLibRawTagReader is a RawTagReader backed by TagLib's tag read —
// sourced from the file's own embedded ID3v2/Vorbis/MP4 tags, no audio
// decode, independent of resolved (AcoustID/MusicBrainz) metadata.
type TagLibRawTagReader struct{}

func NewTagLibRawTagReader() *TagLibRawTagReader {
	return &TagLibRawTagReader{}
}

// ReadRawTags reads path's own embedded title/artist/album/album-artist
// tags. Like TagLibTagger, it reads against path's real, content-sniffed
// format rather than trusting a possibly mismatched extension.
func (r *TagLibRawTagReader) ReadRawTags(ctx context.Context, path string) (usecases.RawTags, error) {
	var result usecases.RawTags
	err := withCorrectExtension(path, func(workingPath string) error {
		tags, err := taglib.ReadTags(workingPath)
		if err != nil {
			return fmt.Errorf("reading tags for %s: %w", path, err)
		}
		result = usecases.RawTags{
			Title:       first(tags[taglib.Title]),
			Artist:      first(tags[taglib.Artist]),
			Album:       first(tags[taglib.Album]),
			AlbumArtist: first(tags[taglib.AlbumArtist]),
		}
		return nil
	})
	if err != nil {
		return usecases.RawTags{}, err
	}
	return result, nil
}
