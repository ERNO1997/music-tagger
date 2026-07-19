package v1

import (
	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// CoverHandler serves a tracked file's stored cover art image bytes.
type CoverHandler struct {
	store usecases.TrackingStore
}

func NewCoverHandler(store usecases.TrackingStore) *CoverHandler {
	return &CoverHandler{store: store}
}

// Serve looks up the cover art path for the file identified by the
// "path" query parameter and streams its bytes. The served path always
// comes from our own tracking store (only ever set by the enrich job to
// paths under /data/covers/), never from client input directly, so there
// is no path-traversal exposure here.
func (h *CoverHandler) Serve(c *fiber.Ctx) error {
	path := c.Query("path")
	if path == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path query parameter is required")
	}

	coverArtPath, found, err := h.store.GetCoverArtPath(c.Context(), path)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if !found {
		return fiber.NewError(fiber.StatusNotFound, "no cover art stored for this file")
	}

	return c.SendFile(coverArtPath)
}
