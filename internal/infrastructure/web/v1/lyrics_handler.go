package v1

import (
	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// LyricsResponse is the JSON representation of a tracked file's stored
// lyrics, per the music-library-scan capability's GET /api/v1/library/lyrics
// contract.
type LyricsResponse struct {
	PlainLyrics  string `json:"plain_lyrics"`
	SyncedLyrics string `json:"synced_lyrics,omitempty"`
}

// LyricsHandler serves a tracked file's stored lyrics.
type LyricsHandler struct {
	store usecases.TrackingStore
}

func NewLyricsHandler(store usecases.TrackingStore) *LyricsHandler {
	return &LyricsHandler{store: store}
}

// Get looks up the lyrics for the file identified by the "path" query
// parameter and returns them as JSON.
func (h *LyricsHandler) Get(c *fiber.Ctx) error {
	path := c.Query("path")
	if path == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path query parameter is required")
	}

	plainLyrics, syncedLyrics, found, err := h.store.GetLyrics(c.Context(), path)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if !found {
		return fiber.NewError(fiber.StatusNotFound, "no lyrics stored for this file")
	}

	return c.JSON(LyricsResponse{PlainLyrics: plainLyrics, SyncedLyrics: syncedLyrics})
}
