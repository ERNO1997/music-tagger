package usecases

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"path/filepath"

	"music-tagger/internal/domain"
)

// ErrAnalysisInProgress is returned by AnalysisManager.Start when an
// analysis pass is already running.
var ErrAnalysisInProgress = ErrJobInProgress

// AnalysisStatus is a snapshot of the current/most recent analysis pass.
type AnalysisStatus = JobStatus

// DestinationComputer computes an already-identified-and-tagged file's
// canonical destination path with no filesystem access — the same
// computation Relocator.Relocate performs before actually moving a file,
// implemented by internal/infrastructure/filestat.PathRelocator. Reused
// here so the passive relocation-detection pass can check whether a file
// is already at its destination without moving it.
type DestinationComputer interface {
	ComputeDestination(path string, meta RelocateInput) string
}

// AnalysisManager coordinates a single background analysis pass — over
// every tracked, non-missing file — at a time, via its own JobManager. It
// never looks anything up from AcoustID/MusicBrainz/Cover Art
// Archive/LRCLIB and never moves or tags a file; it only computes what
// identify's fingerprinting step would, reads a file's own embedded cover
// art/lyrics, and checks a file's current path against its already-
// computed canonical destination. Unlike the other managers, it has no
// user-facing trigger: it is started automatically by RefreshManager after
// every refresh completes. Serialized against an in-progress relocate job,
// for the same reason scan and relocate already exclude each other:
// relocation moves files and updates their tracked path, and a concurrent
// analysis pass reading paths could race with that.
type AnalysisManager struct {
	fingerprinter Fingerprinter
	tagger        Tagger
	coverStore    CoverArtStore
	store         TrackingStore
	destination   DestinationComputer

	job            JobManager
	relocateStatus StatusChecker
}

func NewAnalysisManager(fingerprinter Fingerprinter, tagger Tagger, coverStore CoverArtStore, store TrackingStore, destination DestinationComputer) *AnalysisManager {
	return &AnalysisManager{
		fingerprinter: fingerprinter,
		tagger:        tagger,
		coverStore:    coverStore,
		store:         store,
		destination:   destination,
	}
}

// SetRelocateStatus wires the relocate job's status checker in after both
// managers exist, mirroring RefreshManager.SetRelocateStatus. Must be
// called before the automatic startup scan (and its chained analysis
// pass) is triggered.
func (m *AnalysisManager) SetRelocateStatus(s StatusChecker) {
	m.relocateStatus = s
}

// Start begins an analysis pass over every tracked, non-missing file in
// the background, if no analysis pass is currently running and no
// relocate job is running. It returns ErrAnalysisInProgress or
// ErrBlockedByRelocate accordingly. A per-file failure (fingerprinting,
// embedded-content read, or store write) is logged and does not abort the
// rest of the pass.
func (m *AnalysisManager) Start() error {
	if m.relocateStatus != nil && m.relocateStatus.Status().Running {
		return ErrBlockedByRelocate
	}

	return m.job.Start(func(report func(processed, total int)) {
		ctx := context.Background()

		records, err := m.store.LoadAll(ctx)
		if err != nil {
			log.Printf("analysis pass: loading tracked records: %v", err)
			report(0, 0)
			return
		}

		paths := make([]string, 0, len(records))
		for path, rec := range records {
			if !rec.Missing {
				paths = append(paths, path)
			}
		}

		total := len(paths)
		report(0, total)

		for i, path := range paths {
			m.analyzeOne(ctx, path, records[path])
			report(i+1, total)
		}
	})
}

// Status returns a snapshot of the current/most recent analysis pass.
func (m *AnalysisManager) Status() AnalysisStatus {
	return m.job.Status()
}

func (m *AnalysisManager) analyzeOne(ctx context.Context, path string, rec domain.FileRecord) {
	if m.detectIdentificationFromTags(ctx, path, rec) {
		// The file's status/tagged outcome just changed — re-fetch so the
		// remaining steps (in particular detectRelocated, which requires
		// identified+tagged) see the fresh state within this same pass
		// rather than only catching up on the next refresh cycle.
		fresh, found, err := m.store.Get(ctx, path)
		if err != nil {
			log.Printf("analysis pass: %s: re-fetching after tag-based identification: %v", path, err)
			return
		}
		if !found {
			return
		}
		rec = fresh
	}

	m.fingerprint(ctx, path, rec)
	m.detectEmbeddedContent(ctx, path, rec)
	m.detectRelocated(ctx, path, rec)
}

// fingerprint computes and stores path's Chromaprint fingerprint, using
// the same Fingerprinter mechanism identify calls lazily — only if rec
// doesn't already have one.
func (m *AnalysisManager) fingerprint(ctx context.Context, path string, rec domain.FileRecord) {
	if rec.Fingerprint != "" {
		return
	}

	fp, err := m.fingerprinter.Fingerprint(ctx, path)
	if err != nil {
		// Duration is left at its already-stored value, same as identify's
		// own lazy-fingerprinting failure path.
		if recErr := m.store.RecordFingerprint(ctx, path, "", rec.DurationSeconds, err.Error()); recErr != nil {
			log.Printf("analysis pass: %s: recording fingerprint failure: %v", path, recErr)
		}
		return
	}

	if err := m.store.RecordFingerprint(ctx, path, fp.Chroma, fp.Duration.Seconds(), ""); err != nil {
		log.Printf("analysis pass: %s: recording fingerprint: %v", path, err)
	}
}

// detectEmbeddedContent reads path's own embedded cover art/lyrics and
// stores whichever of rec's CoverArtPath/Lyrics is currently empty —
// leaving an already-stored value (from a prior enrichment or an earlier
// analysis pass) untouched.
func (m *AnalysisManager) detectEmbeddedContent(ctx context.Context, path string, rec domain.FileRecord) {
	if rec.CoverArtPath != "" && rec.Lyrics != "" {
		return
	}

	coverArt, lyrics, err := m.tagger.ReadEmbeddedContent(ctx, path)
	if err != nil {
		log.Printf("analysis pass: %s: reading embedded content: %v", path, err)
		return
	}

	if rec.CoverArtPath == "" && len(coverArt) > 0 {
		if err := m.storeEmbeddedCoverArt(ctx, path, coverArt); err != nil {
			log.Printf("analysis pass: %s: storing embedded cover art: %v", path, err)
		}
	}
	if rec.Lyrics == "" && lyrics != "" {
		if err := m.store.RecordLyrics(ctx, path, lyrics, ""); err != nil {
			log.Printf("analysis pass: %s: recording embedded lyrics: %v", path, err)
		}
	}
}

// storeEmbeddedCoverArt saves a file's own embedded cover image, keyed by
// its content hash rather than a MusicBrainz release — an unidentified
// file has no release to key by. This coincidentally reuses an
// identically-embedded image already saved for another file, but performs
// no deliberate cross-track dedup the way enrichment's Cover Art
// Archive-backed lookup does.
func (m *AnalysisManager) storeEmbeddedCoverArt(ctx context.Context, path string, data []byte) error {
	sum := sha256.Sum256(data)
	key := hex.EncodeToString(sum[:])

	savedPath, exists := m.coverStore.Path(key)
	if !exists {
		var err error
		savedPath, err = m.coverStore.Save(key, data)
		if err != nil {
			return err
		}
	}

	return m.store.RecordCoverArt(ctx, path, savedPath)
}

// detectRelocated marks an identified, tagged, not-yet-relocated file as
// relocated when its current path already equals its computed canonical
// destination — without moving it, consistent with the on-demand
// relocation action's own "already at destination" no-op semantics.
func (m *AnalysisManager) detectRelocated(ctx context.Context, path string, rec domain.FileRecord) {
	if rec.Status != domain.StatusIdentified || !rec.Tagged || rec.Relocated {
		return
	}

	dest := m.destination.ComputeDestination(path, RelocateInput{
		Artist:      rec.Artist,
		Album:       rec.Album,
		Title:       rec.Title,
		TrackNumber: rec.TrackNumber,
		Year:        rec.Year,
	})
	if filepath.Clean(dest) != filepath.Clean(path) {
		return
	}

	if err := m.store.RecordRelocation(ctx, path, path); err != nil {
		log.Printf("analysis pass: %s: marking already-relocated: %v", path, err)
	}
}

// detectIdentificationFromTags treats a file's own embedded MusicBrainz
// recording ID as authoritative over the tracking store: when present
// alongside a non-empty embedded artist and title, and different from the
// file's currently-stored recording MBID (including when nothing is
// stored yet), it records the file as identified from the embedded tags
// and marks it tagged — without calling AcoustID or MusicBrainz. Returns
// true when it changed anything, so the caller can re-fetch rec before
// running the remaining analysis steps.
//
// The comparison against rec.RecordingMBID is deliberate and load-bearing:
// RecordIdentification unconditionally resets cover art, lyrics, tagged,
// and relocated on every call where status becomes identified, since its
// only other caller (on-demand identify) is always a deliberate
// re-identification. Calling it here on every pass regardless of whether
// anything changed would silently wipe tagged/relocated back to false on
// every single pass — so this only calls it when the embedded recording ID
// genuinely disagrees with what's already stored.
func (m *AnalysisManager) detectIdentificationFromTags(ctx context.Context, path string, rec domain.FileRecord) bool {
	embedded, err := m.tagger.ReadEmbeddedTags(ctx, path)
	if err != nil {
		log.Printf("analysis pass: %s: reading embedded tags for identification: %v", path, err)
		return false
	}

	if embedded.RecordingMBID == "" || embedded.Artist == "" || embedded.Title == "" {
		return false
	}
	if embedded.RecordingMBID == rec.RecordingMBID {
		return false
	}

	result := IdentificationResult{
		Status: domain.StatusIdentified,
		Metadata: RecordingMetadata{
			RecordingID:      embedded.RecordingMBID,
			Artist:           embedded.Artist,
			Album:            embedded.Album,
			Title:            embedded.Title,
			TrackNumber:      embedded.TrackNumber,
			AlbumArtist:      embedded.AlbumArtist,
			Year:             embedded.Year,
			DiscNumber:       embedded.DiscNumber,
			ReleaseMBID:      embedded.ReleaseMBID,
			ReleaseGroupMBID: embedded.ReleaseGroupMBID,
			ArtistMBID:       embedded.ArtistMBID,
		},
	}
	if err := m.store.RecordIdentification(ctx, path, result); err != nil {
		log.Printf("analysis pass: %s: recording identification from embedded tags: %v", path, err)
		return false
	}
	if err := m.store.RecordTagged(ctx, path, true, ""); err != nil {
		log.Printf("analysis pass: %s: marking tagged after identification from embedded tags: %v", path, err)
	}
	return true
}
