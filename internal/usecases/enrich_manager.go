package usecases

import (
	"context"
	"log"

	"music-tagger/internal/domain"
)

// ErrEnrichInProgress is returned by EnrichManager.Start when an enrich
// job is already running.
var ErrEnrichInProgress = ErrJobInProgress

// EnrichStatus is a snapshot of the current/most recent enrich job.
type EnrichStatus = JobStatus

// EnrichManager coordinates a single background enrich job (over a list
// of paths) at a time, via its own JobManager — independent of both
// RefreshManager's and IdentifyManager's guards, since enrichment touches
// a different resource (Cover Art Archive) than either local fingerprinting
// or AcoustID/MusicBrainz identification, and none of the three need to
// block each other.
type EnrichManager struct {
	enrich *EnrichFile
	store  TrackingStore
	job    JobManager
}

func NewEnrichManager(enrich *EnrichFile, store TrackingStore) *EnrichManager {
	return &EnrichManager{enrich: enrich, store: store}
}

// Start begins enriching paths in the background if no enrich job is
// currently running. It returns ErrEnrichInProgress otherwise. A path that
// isn't yet identified (no Release MBID available) is skipped, logged,
// and does not abort the rest of the job — same as an unknown path or a
// gateway error.
func (m *EnrichManager) Start(paths []string) error {
	return m.job.Start(func(report func(processed, total int)) {
		total := len(paths)
		report(0, total)

		records, err := m.store.LoadAll(context.Background())
		if err != nil {
			log.Printf("enrich job: loading tracked records: %v", err)
			report(total, total)
			return
		}

		for i, path := range paths {
			rec, ok := records[path]
			switch {
			case !ok:
				log.Printf("enrich job: %s is not a tracked file, skipping", path)
			case rec.Status != domain.StatusIdentified || rec.ReleaseMBID == "":
				log.Printf("enrich job: %s is not yet identified, skipping", path)
			default:
				input := EnrichmentInput{
					Path:                 path,
					ReleaseMBID:          rec.ReleaseMBID,
					ReleaseGroupMBID:     rec.ReleaseGroupMBID,
					Artist:               rec.Artist,
					Title:                rec.Title,
					Album:                rec.Album,
					DurationSeconds:      int(rec.DurationSeconds),
					ExistingCoverArtPath: rec.CoverArtPath,
					ExistingLyrics:       rec.Lyrics,
				}
				if err := m.enrich.Enrich(context.Background(), input); err != nil {
					log.Printf("enrich job: %s: %v", path, err)
				}
			}
			report(i+1, total)
		}
	})
}

// Status returns a snapshot of the current/most recent enrich job.
func (m *EnrichManager) Status() EnrichStatus {
	return m.job.Status()
}
