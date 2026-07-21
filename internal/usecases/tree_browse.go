package usecases

import (
	"context"
	"sort"
	"strings"

	"music-tagger/internal/domain"
)

// DirectorySummary is one immediate subdirectory under a TreeBrowse prefix,
// with aggregate counts over every tracked file beneath it (at any depth).
type DirectorySummary struct {
	Name            string
	TotalCount      int
	IdentifiedCount int
}

// TreeResult is one folder-tree browse response: the immediate
// subdirectories under the browsed prefix, and the tracked files directly
// at that level (already filtered, sorted, and paginated).
type TreeResult struct {
	Directories []DirectorySummary
	Files       []domain.FileRecord
	FilesTotal  int
}

// TreeBrowse groups a prefix-scoped slice of the tracked library into
// immediate subdirectories (with aggregate counts) and direct files at that
// level, reflecting /music's actual on-disk directory structure. Fetches
// everything under prefix in one query and groups it in Go rather than with
// recursive/window-function SQL — bounded in practice by the size of the
// library under that prefix, never large for a directory-organized
// collection even browsing from the root.
type TreeBrowse struct {
	store TrackingStore
}

func NewTreeBrowse(store TrackingStore) *TreeBrowse {
	return &TreeBrowse{store: store}
}

// Browse returns prefix's immediate subdirectories and its direct files,
// honoring filter and sort, with the direct-files list paginated by
// limit/offset.
func (u *TreeBrowse) Browse(ctx context.Context, prefix string, filter LibraryFilter, sortSpec LibrarySort, limit, offset int) (TreeResult, error) {
	records, err := u.store.PathsUnder(ctx, prefix)
	if err != nil {
		return TreeResult{}, err
	}

	trimmedPrefix := strings.TrimSuffix(prefix, "/")

	dirIndex := make(map[string]*DirectorySummary)
	var dirOrder []string
	var directFiles []domain.FileRecord

	for _, rec := range records {
		if !matchesLibraryFilter(rec, filter) {
			continue
		}
		rest := strings.TrimPrefix(rec.Path, trimmedPrefix)
		rest = strings.TrimPrefix(rest, "/")
		segments := strings.SplitN(rest, "/", 2)
		if len(segments) < 2 || segments[1] == "" {
			directFiles = append(directFiles, rec)
			continue
		}
		dirName := segments[0]
		summary, ok := dirIndex[dirName]
		if !ok {
			summary = &DirectorySummary{Name: dirName}
			dirIndex[dirName] = summary
			dirOrder = append(dirOrder, dirName)
		}
		summary.TotalCount++
		if rec.EffectiveStatus() == domain.StatusIdentified {
			summary.IdentifiedCount++
		}
	}

	sort.Strings(dirOrder)
	directories := make([]DirectorySummary, 0, len(dirOrder))
	for _, name := range dirOrder {
		directories = append(directories, *dirIndex[name])
	}

	sortFileRecords(directFiles, sortSpec)
	total := len(directFiles)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return TreeResult{Directories: directories, Files: directFiles[offset:end], FilesTotal: total}, nil
}

// matchesLibraryFilter reports whether rec matches filter, mirroring
// buildLibraryWhere's SQL semantics for records already fetched into
// memory (PathsUnder itself takes no filter — TreeBrowse applies it here).
func matchesLibraryFilter(rec domain.FileRecord, filter LibraryFilter) bool {
	if filter.Status != "" && string(rec.EffectiveStatus()) != filter.Status {
		return false
	}
	if filter.Tagged != nil && rec.Tagged != *filter.Tagged {
		return false
	}
	if filter.Relocated != nil && rec.Relocated != *filter.Relocated {
		return false
	}
	if filter.HasLyrics != nil {
		hasLyrics := rec.Lyrics != "" || rec.SyncedLyrics != ""
		if hasLyrics != *filter.HasLyrics {
			return false
		}
	}
	if filter.HasCoverArt != nil {
		hasCoverArt := rec.CoverArtPath != ""
		if hasCoverArt != *filter.HasCoverArt {
			return false
		}
	}
	if filter.Search != "" {
		q := strings.ToLower(filter.Search)
		haystacks := []string{rec.Path, rec.Artist, rec.Album, rec.Title, rec.RawTitle, rec.RawArtist, rec.RawAlbum}
		matched := false
		for _, h := range haystacks {
			if strings.Contains(strings.ToLower(h), q) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// sortFileRecords sorts records in place per sortSpec, mirroring QueryPage's
// SQL ordering (primary field per sortSpec.Desc, always-ascending path
// tie-break) for records already fetched into memory.
func sortFileRecords(records []domain.FileRecord, sortSpec LibrarySort) {
	sort.SliceStable(records, func(i, j int) bool {
		c := compareByLibrarySortField(records[i], records[j], sortSpec.By)
		if c != 0 {
			if sortSpec.Desc {
				return c > 0
			}
			return c < 0
		}
		return records[i].Path < records[j].Path
	})
}

func compareByLibrarySortField(a, b domain.FileRecord, by string) int {
	switch by {
	case "status":
		return strings.Compare(string(a.EffectiveStatus()), string(b.EffectiveStatus()))
	case "artist":
		return strings.Compare(a.Artist, b.Artist)
	case "album":
		return strings.Compare(a.Album, b.Album)
	case "duration":
		switch {
		case a.DurationSeconds < b.DurationSeconds:
			return -1
		case a.DurationSeconds > b.DurationSeconds:
			return 1
		default:
			return 0
		}
	case "year":
		return a.Year - b.Year
	default:
		return 0
	}
}
