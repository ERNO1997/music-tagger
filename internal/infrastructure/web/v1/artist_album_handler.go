package v1

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// ArtistEntry is one distinct artist grouping, per GET /api/v1/library/artists.
type ArtistEntry struct {
	ArtistKey      string   `json:"artist_key"`
	Artist         string   `json:"artist"`
	TrackCount     int      `json:"track_count"`
	NameMismatch   bool     `json:"name_mismatch,omitempty"`
	LabelCollision bool     `json:"label_collision,omitempty"`
	DistinctNames  []string `json:"distinct_names,omitempty"`
}

// ArtistsListResponse is the JSON representation of GET /api/v1/library/artists.
type ArtistsListResponse struct {
	Artists []ArtistEntry `json:"artists"`
}

// AlbumEntry is one distinct album grouping for an artist, per
// GET /api/v1/library/albums.
type AlbumEntry struct {
	AlbumKey       string   `json:"album_key"`
	Album          string   `json:"album"`
	TrackCount     int      `json:"track_count"`
	NameMismatch   bool     `json:"name_mismatch,omitempty"`
	LabelCollision bool     `json:"label_collision,omitempty"`
	DistinctNames  []string `json:"distinct_names,omitempty"`
}

// AlbumsListResponse is the JSON representation of GET /api/v1/library/albums.
type AlbumsListResponse struct {
	Albums []AlbumEntry `json:"albums"`
}

// TracksListResponse is the JSON representation of GET /api/v1/library/tracks.
type TracksListResponse struct {
	Entries []LibraryEntry `json:"entries"`
}

// MissingAlbumEntry is one MusicBrainz album not present in the local
// library, per GET /api/v1/library/artists/completeness.
type MissingAlbumEntry struct {
	Title string `json:"title"`
	Year  int    `json:"year,omitempty"`
}

// ArtistCompletenessResponse is the JSON representation of
// GET /api/v1/library/artists/completeness.
type ArtistCompletenessResponse struct {
	OwnedAlbums int                 `json:"owned_albums"`
	TotalAlbums int                 `json:"total_albums"`
	Missing     []MissingAlbumEntry `json:"missing"`
}

// MissingTrackEntry is one MusicBrainz track not present in the local
// library, per GET /api/v1/library/albums/completeness.
type MissingTrackEntry struct {
	Title       string `json:"title"`
	TrackNumber int    `json:"track_number,omitempty"`
}

// AlbumCompletenessResponse is the JSON representation of
// GET /api/v1/library/albums/completeness.
type AlbumCompletenessResponse struct {
	OwnedTracks     int                 `json:"owned_tracks"`
	TotalTracks     int                 `json:"total_tracks"`
	Missing         []MissingTrackEntry `json:"missing"`
	ReleaseMismatch bool                `json:"release_mismatch,omitempty"`
}

// ArtistAlbumHandler serves the Artist -> Album -> Track browsing
// endpoints, grouping by resolved metadata with a raw-tag fallback so
// unidentified files still appear, plus MusicBrainz completeness checks for
// MBID-keyed groupings.
type ArtistAlbumHandler struct {
	store        usecases.TrackingStore
	completeness *usecases.CompletenessChecker
}

func NewArtistAlbumHandler(store usecases.TrackingStore, completeness *usecases.CompletenessChecker) *ArtistAlbumHandler {
	return &ArtistAlbumHandler{store: store, completeness: completeness}
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
		entries = append(entries, ArtistEntry{
			ArtistKey:      a.Key,
			Artist:         a.Artist,
			TrackCount:     a.TrackCount,
			NameMismatch:   a.NameMismatch,
			LabelCollision: a.LabelCollision,
			DistinctNames:  a.DistinctNames,
		})
	}
	return c.JSON(ArtistsListResponse{Artists: entries})
}

// resolveArtistKey returns the request's "artist_key" query parameter
// directly, falling back to resolving its "artist" name parameter for
// backward compatibility. Fails if neither is present.
func (h *ArtistAlbumHandler) resolveArtistKey(c *fiber.Ctx) (string, error) {
	if key := c.Query("artist_key"); key != "" {
		return key, nil
	}
	name := c.Query("artist")
	if name == "" {
		return "", fiber.NewError(fiber.StatusBadRequest, "artist_key or artist query parameter is required")
	}
	key, err := h.store.ResolveArtistKey(c.Context(), name)
	if err != nil {
		return "", fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return key, nil
}

// resolveAlbumKey returns the request's "album_key" query parameter
// directly, falling back to resolving its "album" name parameter (scoped to
// artistKey) for backward compatibility. Fails if neither is present.
func (h *ArtistAlbumHandler) resolveAlbumKey(c *fiber.Ctx, artistKey string) (string, error) {
	if key := c.Query("album_key"); key != "" {
		return key, nil
	}
	name := c.Query("album")
	if name == "" {
		return "", fiber.NewError(fiber.StatusBadRequest, "album_key or album query parameter is required")
	}
	key, err := h.store.ResolveAlbumKey(c.Context(), artistKey, name)
	if err != nil {
		return "", fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return key, nil
}

// Albums returns every distinct album grouping for the "artist_key" (or,
// for backward compatibility, "artist" name) query parameter, honoring the
// request's filter query parameters.
func (h *ArtistAlbumHandler) Albums(c *fiber.Ctx) error {
	artistKey, err := h.resolveArtistKey(c)
	if err != nil {
		return err
	}

	filter, err := parseLibraryFilter(c)
	if err != nil {
		return err
	}

	albums, err := h.store.ListAlbums(c.Context(), artistKey, filter)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	entries := make([]AlbumEntry, 0, len(albums))
	for _, a := range albums {
		entries = append(entries, AlbumEntry{
			AlbumKey:       a.Key,
			Album:          a.Album,
			TrackCount:     a.TrackCount,
			NameMismatch:   a.NameMismatch,
			LabelCollision: a.LabelCollision,
			DistinctNames:  a.DistinctNames,
		})
	}
	return c.JSON(AlbumsListResponse{Albums: entries})
}

// Tracks returns the "artist_key"+"album_key" (or, for backward
// compatibility, "artist"+"album" name) query parameters' matching tracks,
// sorted by track number, honoring the request's filter query parameters.
func (h *ArtistAlbumHandler) Tracks(c *fiber.Ctx) error {
	artistKey, err := h.resolveArtistKey(c)
	if err != nil {
		return err
	}
	albumKey, err := h.resolveAlbumKey(c, artistKey)
	if err != nil {
		return err
	}

	filter, err := parseLibraryFilter(c)
	if err != nil {
		return err
	}

	records, err := h.store.ListTracks(c.Context(), artistKey, albumKey, filter)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	entries := make([]LibraryEntry, 0, len(records))
	for _, r := range records {
		entries = append(entries, libraryEntryFrom(r))
	}
	return c.JSON(TracksListResponse{Entries: entries})
}

// ArtistCompletenessCheck compares an artist grouping's locally-owned
// albums against its full MusicBrainz discography. Available only for an
// MBID-keyed grouping; "refresh=true" bypasses the short in-process cache
// for a manual recheck.
func (h *ArtistAlbumHandler) ArtistCompletenessCheck(c *fiber.Ctx) error {
	artistKey, err := h.resolveArtistKey(c)
	if err != nil {
		return err
	}

	result, err := h.completeness.ArtistCompleteness(c.Context(), artistKey, c.QueryBool("refresh", false))
	if err != nil {
		if errors.Is(err, usecases.ErrCompletenessUnavailable) {
			return fiber.NewError(fiber.StatusUnprocessableEntity, err.Error())
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	missing := make([]MissingAlbumEntry, 0, len(result.Missing))
	for _, m := range result.Missing {
		missing = append(missing, MissingAlbumEntry{Title: m.Title, Year: m.Year})
	}
	return c.JSON(ArtistCompletenessResponse{
		OwnedAlbums: result.OwnedAlbums,
		TotalAlbums: result.TotalAlbums,
		Missing:     missing,
	})
}

// AlbumCompletenessCheck compares an album grouping's locally-owned tracks
// against its MusicBrainz release tracklist. Available only for an
// MBID-keyed grouping; "refresh=true" bypasses the short in-process cache
// for a manual recheck.
func (h *ArtistAlbumHandler) AlbumCompletenessCheck(c *fiber.Ctx) error {
	artistKey, err := h.resolveArtistKey(c)
	if err != nil {
		return err
	}
	albumKey, err := h.resolveAlbumKey(c, artistKey)
	if err != nil {
		return err
	}

	result, err := h.completeness.AlbumCompleteness(c.Context(), artistKey, albumKey, c.QueryBool("refresh", false))
	if err != nil {
		if errors.Is(err, usecases.ErrCompletenessUnavailable) {
			return fiber.NewError(fiber.StatusUnprocessableEntity, err.Error())
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	missing := make([]MissingTrackEntry, 0, len(result.Missing))
	for _, m := range result.Missing {
		missing = append(missing, MissingTrackEntry{Title: m.Title, TrackNumber: m.TrackNumber})
	}
	return c.JSON(AlbumCompletenessResponse{
		OwnedTracks:     result.OwnedTracks,
		TotalTracks:     result.TotalTracks,
		Missing:         missing,
		ReleaseMismatch: result.ReleaseMismatch,
	})
}
