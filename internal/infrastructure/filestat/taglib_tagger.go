package filestat

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	taglib "go.senan.xyz/taglib"

	"music-tagger/internal/usecases"
)

// TagLibTagger is a Tagger backed by TagLib (via a pure-Go, CGO-free Wasm
// binding). A single implementation handles MP3/FLAC/M4A uniformly —
// TagLib determines the file's format itself and maps these normalized
// tag keys to the correct underlying representation: ID3v2 frames for
// MP3, Vorbis comments for FLAC, MP4 atoms for M4A.
type TagLibTagger struct{}

func NewTagLibTagger() *TagLibTagger {
	return &TagLibTagger{}
}

// Tag writes meta into path's own tags. Writing uses TagLib's default
// (merge) behavior rather than the Clear option, so existing tag data not
// covered by TagInput is preserved untouched. If path's extension doesn't
// match its real, content-sniffed format, tagging is performed against
// the correct format instead — see withCorrectExtension.
func (t *TagLibTagger) Tag(ctx context.Context, path string, meta usecases.TagInput) error {
	return withCorrectExtension(path, func(workingPath string) error {
		tags := map[string][]string{}
		setIfNonEmpty(tags, taglib.Title, meta.Title)
		setIfNonEmpty(tags, taglib.Artist, meta.Artist)
		setIfNonEmpty(tags, taglib.Album, meta.Album)
		setIfNonEmpty(tags, taglib.AlbumArtist, meta.AlbumArtist)
		if track := formatNumberPair(meta.TrackNumber, meta.TotalTracks); track != "" {
			tags[taglib.TrackNumber] = []string{track}
		}
		if disc := formatNumberPair(meta.DiscNumber, meta.TotalDiscs); disc != "" {
			tags[taglib.DiscNumber] = []string{disc}
		}
		if meta.Year > 0 {
			tags[taglib.Date] = []string{strconv.Itoa(meta.Year)}
		}
		if meta.Lyrics != "" {
			tags[taglib.Lyrics] = []string{meta.Lyrics}
		}

		if err := taglib.WriteTags(workingPath, tags, 0); err != nil {
			return fmt.Errorf("writing tags for %s: %w", path, err)
		}

		if meta.CoverArt != nil {
			if err := taglib.WriteImage(workingPath, meta.CoverArt); err != nil {
				return fmt.Errorf("writing cover art for %s: %w", path, err)
			}
		}

		return nil
	})
}

// ReadEmbeddedTags reads path's actual, currently-embedded tags live from
// disk, independent of any cached tracking-store state. Like Tag, it
// reads against path's real, content-sniffed format rather than trusting
// a possibly-mismatched extension.
func (t *TagLibTagger) ReadEmbeddedTags(ctx context.Context, path string) (usecases.EmbeddedTags, error) {
	var result usecases.EmbeddedTags
	err := withCorrectExtension(path, func(workingPath string) error {
		tags, err := taglib.ReadTags(workingPath)
		if err != nil {
			return fmt.Errorf("reading tags for %s: %w", path, err)
		}

		props, err := taglib.ReadProperties(workingPath)
		if err != nil {
			return fmt.Errorf("reading properties for %s: %w", path, err)
		}

		result = usecases.EmbeddedTags{
			Title:       first(tags[taglib.Title]),
			Artist:      first(tags[taglib.Artist]),
			Album:       first(tags[taglib.Album]),
			AlbumArtist: first(tags[taglib.AlbumArtist]),
			TrackNumber: parseLeadingNumber(first(tags[taglib.TrackNumber])),
			DiscNumber:  parseLeadingNumber(first(tags[taglib.DiscNumber])),
			Year:        parseLeadingNumber(first(tags[taglib.Date])),
			HasLyrics:   first(tags[taglib.Lyrics]) != "",
			HasCoverArt: len(props.Images) > 0,
		}
		return nil
	})
	if err != nil {
		return usecases.EmbeddedTags{}, err
	}
	return result, nil
}

// ReadEmbeddedContent reads path's actual embedded cover image bytes and
// lyrics text live from disk — the same underlying TagLib read
// ReadEmbeddedTags summarizes as booleans. coverArt is nil and lyrics is
// empty when absent, not an error.
func (t *TagLibTagger) ReadEmbeddedContent(ctx context.Context, path string) (coverArt []byte, lyrics string, err error) {
	err = withCorrectExtension(path, func(workingPath string) error {
		tags, terr := taglib.ReadTags(workingPath)
		if terr != nil {
			return fmt.Errorf("reading tags for %s: %w", path, terr)
		}
		lyrics = first(tags[taglib.Lyrics])

		img, ierr := taglib.ReadImage(workingPath)
		if ierr != nil {
			return fmt.Errorf("reading embedded image for %s: %w", path, ierr)
		}
		if len(img) > 0 {
			coverArt = img
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return coverArt, lyrics, nil
}

func setIfNonEmpty(tags map[string][]string, key, value string) {
	if value != "" {
		tags[key] = []string{value}
	}
}

// formatNumberPair renders a track/disc number as "n/total" when a total
// is known, or bare "n" otherwise. A non-positive n (unresolved) yields an
// empty string, so the caller skips writing that tag entirely rather than
// embedding a meaningless "0".
func formatNumberPair(n, total int) string {
	if n <= 0 {
		return ""
	}
	if total > 0 {
		return fmt.Sprintf("%d/%d", n, total)
	}
	return strconv.Itoa(n)
}

// parseLeadingNumber parses the leading integer of a possibly "n/total"
// formatted tag value, e.g. "5/12" -> 5. Returns 0 for an empty or
// unparseable value.
func parseLeadingNumber(value string) int {
	if value == "" {
		return 0
	}
	head, _, _ := strings.Cut(value, "/")
	n, err := strconv.Atoi(strings.TrimSpace(head))
	if err != nil {
		return 0
	}
	return n
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
