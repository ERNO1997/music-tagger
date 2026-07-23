package usecases

import (
	"sort"
	"strings"
)

// ArtistRow is one tracked file's artist-identifying fields, as fetched by
// SQLiteStore.ListArtists for grouping.
type ArtistRow struct {
	Artist     string
	RawArtist  string
	ArtistMBID string
}

// AlbumRow is one tracked file's album-identifying fields, as fetched by
// SQLiteStore.ListAlbums for grouping, already scoped by its caller to a
// single artist grouping.
type AlbumRow struct {
	Album            string
	RawAlbum         string
	ReleaseGroupMBID string
}

// groupedRow is the generic (name, rawName, mbid) shape groupRows operates
// on, shared by both the artist and album grouping dimensions.
type groupedRow struct {
	name    string
	rawName string
	mbid    string
}

// groupResult is the generic shape groupRows produces before being
// converted to ArtistSummary/AlbumSummary.
type groupResult struct {
	key            string
	label          string
	trackCount     int
	nameMismatch   bool
	labelCollision bool
	distinctNames  []string
}

// GroupArtists computes artist groupings (key, representative label, track
// count, and mismatch flags) from raw per-file artist rows. See groupRows
// for the grouping/mismatch rules this applies.
func GroupArtists(rows []ArtistRow) []ArtistSummary {
	generic := make([]groupedRow, len(rows))
	for i, r := range rows {
		generic[i] = groupedRow{name: r.Artist, rawName: r.RawArtist, mbid: r.ArtistMBID}
	}

	grouped := groupRows(generic, UnknownArtist)
	summaries := make([]ArtistSummary, len(grouped))
	for i, g := range grouped {
		summaries[i] = ArtistSummary{
			Key:            g.key,
			Artist:         g.label,
			TrackCount:     g.trackCount,
			NameMismatch:   g.nameMismatch,
			LabelCollision: g.labelCollision,
			DistinctNames:  g.distinctNames,
		}
	}
	return summaries
}

// GroupAlbums computes album groupings (key, representative label, track
// count, and mismatch flags) from raw per-file album rows already scoped to
// one artist grouping. See groupRows for the grouping/mismatch rules this
// applies.
func GroupAlbums(rows []AlbumRow) []AlbumSummary {
	generic := make([]groupedRow, len(rows))
	for i, r := range rows {
		generic[i] = groupedRow{name: r.Album, rawName: r.RawAlbum, mbid: r.ReleaseGroupMBID}
	}

	grouped := groupRows(generic, UnknownAlbum)
	summaries := make([]AlbumSummary, len(grouped))
	for i, g := range grouped {
		summaries[i] = AlbumSummary{
			Key:            g.key,
			Album:          g.label,
			TrackCount:     g.trackCount,
			NameMismatch:   g.nameMismatch,
			LabelCollision: g.labelCollision,
			DistinctNames:  g.distinctNames,
		}
	}
	return summaries
}

// groupRows computes grouping key, representative label, and mismatch flags
// for a set of (name, rawName, mbid) rows, per design.md decisions 2 and 4:
//
//   - Grouping key: mbid when non-empty, else "name:" + the resolved name
//     (COALESCE(name, rawName, unknown)) — mirroring the SQL fallback the
//     store used before grouping moved into Go.
//   - Representative label: the most-frequent non-blank resolved name
//     observed in the group (an MBID-keyed group has no display string of
//     its own).
//   - name_mismatch: a group's files disagree on the resolved name despite
//     sharing one key (only possible for MBID-keyed groups).
//   - label_collision: two different keys resolve to the same display
//     label (case-insensitive) — the two groups are distinct, but a viewer
//     can't tell them apart by label alone.
//
// Neither flag is resolved silently; both are surfaced on every group they
// apply to, per the "if there is a mismatch I want to know it" requirement.
func groupRows(rows []groupedRow, unknown string) []groupResult {
	type state struct {
		key        string
		nameCounts map[string]int
		trackCount int
	}

	groups := map[string]*state{}
	var order []string

	for _, r := range rows {
		name := r.name
		if name == "" {
			name = r.rawName
		}
		if name == "" {
			name = unknown
		}
		key := r.mbid
		if key == "" {
			key = "name:" + name
		}

		g, ok := groups[key]
		if !ok {
			g = &state{key: key, nameCounts: map[string]int{}}
			groups[key] = g
			order = append(order, key)
		}
		g.nameCounts[name]++
		g.trackCount++
	}

	results := make([]groupResult, 0, len(order))
	labelToKeys := map[string][]string{}
	for _, key := range order {
		g := groups[key]
		label := representativeLabel(g.nameCounts)

		var distinct []string
		if len(g.nameCounts) > 1 {
			for name := range g.nameCounts {
				distinct = append(distinct, name)
			}
			sort.Strings(distinct)
		}

		results = append(results, groupResult{
			key:           key,
			label:         label,
			trackCount:    g.trackCount,
			nameMismatch:  len(g.nameCounts) > 1,
			distinctNames: distinct,
		})
		labelKey := strings.ToLower(label)
		labelToKeys[labelKey] = append(labelToKeys[labelKey], key)
	}

	colliding := map[string]bool{}
	for _, keys := range labelToKeys {
		if len(keys) > 1 {
			for _, k := range keys {
				colliding[k] = true
			}
		}
	}
	for i := range results {
		if colliding[results[i].key] {
			results[i].labelCollision = true
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].label) < strings.ToLower(results[j].label)
	})

	return results
}

// representativeLabel picks the most-frequent non-blank name observed in a
// group, ties broken alphabetically (case-insensitive).
func representativeLabel(counts map[string]int) string {
	best := ""
	bestCount := -1
	for name, count := range counts {
		if count > bestCount || (count == bestCount && strings.ToLower(name) < strings.ToLower(best)) {
			best, bestCount = name, count
		}
	}
	return best
}
