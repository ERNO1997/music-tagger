package v1

import (
	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// EmbeddedTagsResponse is the JSON representation of a tracked file's
// actual, currently-embedded tags, read live from disk, per the
// music-library-scan capability's GET /api/v1/library/tags contract.
type EmbeddedTagsResponse struct {
	Title       string `json:"title,omitempty"`
	Artist      string `json:"artist,omitempty"`
	Album       string `json:"album,omitempty"`
	AlbumArtist string `json:"album_artist,omitempty"`
	TrackNumber int    `json:"track_number,omitempty"`
	DiscNumber  int    `json:"disc_number,omitempty"`
	Year        int    `json:"year,omitempty"`

	RecordingMBID    string `json:"recording_mbid,omitempty"`
	ReleaseMBID      string `json:"release_mbid,omitempty"`
	ReleaseGroupMBID string `json:"release_group_mbid,omitempty"`
	ArtistMBID       string `json:"artist_mbid,omitempty"`

	HasLyrics   bool `json:"has_lyrics"`
	HasCoverArt bool `json:"has_cover_art"`
}

// EmbeddedTagsHandler serves a tracked file's actual embedded tags, read
// directly from the physical file — independent of the resolved metadata
// cached in the tracking store — so a user can visually verify what was
// really written by tagging.
type EmbeddedTagsHandler struct {
	tag *usecases.TagFile
}

func NewEmbeddedTagsHandler(tag *usecases.TagFile) *EmbeddedTagsHandler {
	return &EmbeddedTagsHandler{tag: tag}
}

// Get looks up the embedded tags for the file identified by the "path"
// query parameter and returns them as JSON.
func (h *EmbeddedTagsHandler) Get(c *fiber.Ctx) error {
	path := c.Query("path")
	if path == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path query parameter is required")
	}

	tags, found, err := h.tag.GetEmbeddedTags(c.Context(), path)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if !found {
		return fiber.NewError(fiber.StatusNotFound, "file is not tracked or is currently missing from disk")
	}

	return c.JSON(EmbeddedTagsResponse{
		Title:            tags.Title,
		Artist:           tags.Artist,
		Album:            tags.Album,
		AlbumArtist:      tags.AlbumArtist,
		TrackNumber:      tags.TrackNumber,
		DiscNumber:       tags.DiscNumber,
		Year:             tags.Year,
		RecordingMBID:    tags.RecordingMBID,
		ReleaseMBID:      tags.ReleaseMBID,
		ReleaseGroupMBID: tags.ReleaseGroupMBID,
		ArtistMBID:       tags.ArtistMBID,
		HasLyrics:        tags.HasLyrics,
		HasCoverArt:      tags.HasCoverArt,
	})
}
