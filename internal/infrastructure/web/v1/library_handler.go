package v1

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/domain"
	"music-tagger/internal/usecases"
)

// defaultLibraryLimit and maxLibraryLimit bound the "limit" query
// parameter: defaulted when absent, capped when oversized, so a single
// request can never force loading the whole table.
const (
	defaultLibraryLimit = 50
	maxLibraryLimit     = 500
)

// LibraryEntry is the JSON representation of one tracked file, per the
// music-library-scan capability's GET /api/v1/library contract.
type LibraryEntry struct {
	Path            string  `json:"path"`
	Format          string  `json:"format"`
	DurationSeconds float64 `json:"duration_seconds"`
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

// LibraryListResponse is the JSON envelope for GET /api/v1/library: a page
// of entries alongside the total count of records matching the request's
// filter, independent of the page size.
type LibraryListResponse struct {
	Total   int            `json:"total"`
	Entries []LibraryEntry `json:"entries"`
}

// LibraryHandler serves the current tracked state read directly from the
// TrackingStore — no disk walk or fingerprinting happens on this path.
type LibraryHandler struct {
	store usecases.TrackingStore
}

func NewLibraryHandler(store usecases.TrackingStore) *LibraryHandler {
	return &LibraryHandler{store: store}
}

// List returns a filtered, sorted, paginated page of tracked records.
// Query parameters: status, tagged, relocated, q (search), sort, order
// (asc/desc), limit, offset.
func (h *LibraryHandler) List(c *fiber.Ctx) error {
	filter := usecases.LibraryFilter{
		Status: c.Query("status"),
		Search: c.Query("q"),
	}
	if v := c.Query("tagged"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "tagged must be a boolean")
		}
		filter.Tagged = &b
	}
	if v := c.Query("relocated"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "relocated must be a boolean")
		}
		filter.Relocated = &b
	}

	sort := usecases.LibrarySort{
		By:   c.Query("sort"),
		Desc: c.Query("order") == "desc",
	}

	limit := defaultLibraryLimit
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return fiber.NewError(fiber.StatusBadRequest, "limit must be a positive integer")
		}
		limit = n
	}
	if limit > maxLibraryLimit {
		limit = maxLibraryLimit
	}

	offset := 0
	if v := c.Query("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return fiber.NewError(fiber.StatusBadRequest, "offset must be a non-negative integer")
		}
		offset = n
	}

	records, total, err := h.store.QueryPage(c.Context(), filter, sort, limit, offset)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	entries := make([]LibraryEntry, 0, len(records))
	for _, r := range records {
		entries = append(entries, libraryEntryFrom(r))
	}

	return c.JSON(LibraryListResponse{Total: total, Entries: entries})
}

func libraryEntryFrom(r domain.FileRecord) LibraryEntry {
	return LibraryEntry{
		Path:            r.Path,
		Format:          string(r.Format),
		DurationSeconds: r.DurationSeconds,
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
	}
}
