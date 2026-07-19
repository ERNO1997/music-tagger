package v1

import (
	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// LibraryEntry is the JSON representation of one tracked file, per the
// music-library-scan capability's GET /api/v1/library contract.
type LibraryEntry struct {
	Path            string  `json:"path"`
	Format          string  `json:"format"`
	DurationSeconds float64 `json:"duration_seconds"`
	Fingerprint     string  `json:"fingerprint"`
	Status          string  `json:"status"`
	Error           string  `json:"error,omitempty"`
	Artist          string  `json:"artist,omitempty"`
	Album           string  `json:"album,omitempty"`
	Title           string  `json:"title,omitempty"`
	TrackNumber     int     `json:"track_number,omitempty"`
	RecordingMBID   string  `json:"recording_mbid,omitempty"`

	AlbumArtist      string `json:"album_artist,omitempty"`
	Year             int    `json:"year,omitempty"`
	DiscNumber       int    `json:"disc_number,omitempty"`
	TotalDiscs       int    `json:"total_discs,omitempty"`
	TotalTracks      int    `json:"total_tracks,omitempty"`
	ReleaseMBID      string `json:"release_mbid,omitempty"`
	ReleaseGroupMBID string `json:"release_group_mbid,omitempty"`
	ArtistMBID       string `json:"artist_mbid,omitempty"`

	HasCoverArt bool `json:"has_cover_art,omitempty"`
	HasLyrics   bool `json:"has_lyrics,omitempty"`

	Tagged   bool   `json:"tagged,omitempty"`
	TagError string `json:"tag_error,omitempty"`

	Relocated     bool   `json:"relocated,omitempty"`
	RelocateError string `json:"relocate_error,omitempty"`
}

// LibraryHandler serves the current tracked state read directly from the
// TrackingStore — no disk walk or fingerprinting happens on this path.
type LibraryHandler struct {
	store usecases.TrackingStore
}

func NewLibraryHandler(store usecases.TrackingStore) *LibraryHandler {
	return &LibraryHandler{store: store}
}

func (h *LibraryHandler) List(c *fiber.Ctx) error {
	records, err := h.store.LoadAll(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	entries := make([]LibraryEntry, 0, len(records))
	for _, r := range records {
		entries = append(entries, LibraryEntry{
			Path:            r.Path,
			Format:          string(r.Format),
			DurationSeconds: r.DurationSeconds,
			Fingerprint:     r.Fingerprint,
			Status:          string(r.EffectiveStatus()),
			Error:           r.FingerprintError,
			Artist:          r.Artist,
			Album:           r.Album,
			Title:           r.Title,
			TrackNumber:     r.TrackNumber,
			RecordingMBID:   r.RecordingMBID,

			AlbumArtist:      r.AlbumArtist,
			Year:             r.Year,
			DiscNumber:       r.DiscNumber,
			TotalDiscs:       r.TotalDiscs,
			TotalTracks:      r.TotalTracks,
			ReleaseMBID:      r.ReleaseMBID,
			ReleaseGroupMBID: r.ReleaseGroupMBID,
			ArtistMBID:       r.ArtistMBID,

			HasCoverArt: r.CoverArtPath != "",
			HasLyrics:   r.Lyrics != "" || r.SyncedLyrics != "",

			Tagged:   r.Tagged,
			TagError: r.TagError,

			Relocated:     r.Relocated,
			RelocateError: r.RelocateError,
		})
	}

	return c.JSON(entries)
}
