package v1

import (
	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// ArtistEntry is one distinct artist grouping, per GET /api/v1/library/artists.
type ArtistEntry struct {
	Artist     string `json:"artist"`
	TrackCount int    `json:"track_count"`
}

// ArtistsListResponse is the JSON representation of GET /api/v1/library/artists.
type ArtistsListResponse struct {
	Artists []ArtistEntry `json:"artists"`
}

// AlbumEntry is one distinct album grouping for an artist, per
// GET /api/v1/library/albums.
type AlbumEntry struct {
	Album      string `json:"album"`
	TrackCount int    `json:"track_count"`
}

// AlbumsListResponse is the JSON representation of GET /api/v1/library/albums.
type AlbumsListResponse struct {
	Albums []AlbumEntry `json:"albums"`
}

// TracksListResponse is the JSON representation of GET /api/v1/library/tracks.
type TracksListResponse struct {
	Entries []LibraryEntry `json:"entries"`
}

// ArtistAlbumHandler serves the Artist -> Album -> Track browsing
// endpoints, grouping by resolved metadata with a raw-tag fallback so
// unidentified files still appear.
type ArtistAlbumHandler struct {
	store usecases.TrackingStore
}

func NewArtistAlbumHandler(store usecases.TrackingStore) *ArtistAlbumHandler {
	return &ArtistAlbumHandler{store: store}
}

// Artists returns every distinct artist grouping honoring the request's
// filter query parameters.
func (h *ArtistAlbumHandler) Artists(c *fiber.Ctx) error {
	filter, err := parseLibraryFilter(c)
	if err != nil {
		return err
	}

	artists, err := h.store.ListArtists(c.Context(), filter)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	entries := make([]ArtistEntry, 0, len(artists))
	for _, a := range artists {
		entries = append(entries, ArtistEntry{Artist: a.Artist, TrackCount: a.TrackCount})
	}
	return c.JSON(ArtistsListResponse{Artists: entries})
}

// Albums returns every distinct album grouping for the "artist" query
// parameter, honoring the request's filter query parameters.
func (h *ArtistAlbumHandler) Albums(c *fiber.Ctx) error {
	artist := c.Query("artist")
	if artist == "" {
		return fiber.NewError(fiber.StatusBadRequest, "artist query parameter is required")
	}

	filter, err := parseLibraryFilter(c)
	if err != nil {
		return err
	}

	albums, err := h.store.ListAlbums(c.Context(), artist, filter)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	entries := make([]AlbumEntry, 0, len(albums))
	for _, a := range albums {
		entries = append(entries, AlbumEntry{Album: a.Album, TrackCount: a.TrackCount})
	}
	return c.JSON(AlbumsListResponse{Albums: entries})
}

// Tracks returns the "artist"+"album" query parameters' matching tracks,
// sorted by track number, honoring the request's filter query parameters.
func (h *ArtistAlbumHandler) Tracks(c *fiber.Ctx) error {
	artist := c.Query("artist")
	album := c.Query("album")
	if artist == "" || album == "" {
		return fiber.NewError(fiber.StatusBadRequest, "artist and album query parameters are required")
	}

	filter, err := parseLibraryFilter(c)
	if err != nil {
		return err
	}

	records, err := h.store.ListTracks(c.Context(), artist, album, filter)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	entries := make([]LibraryEntry, 0, len(records))
	for _, r := range records {
		entries = append(entries, libraryEntryFrom(r))
	}
	return c.JSON(TracksListResponse{Entries: entries})
}
