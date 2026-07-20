package v1

import (
	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// FingerprintResponse is the JSON representation of a tracked file's
// fingerprint, per GET /api/v1/library/fingerprint.
type FingerprintResponse struct {
	Fingerprint string `json:"fingerprint"`
}

// FingerprintHandler serves a tracked file's stored fingerprint on demand —
// moved off the list endpoint since it's several KB per row and only ever
// shown in the details view.
type FingerprintHandler struct {
	store usecases.TrackingStore
}

func NewFingerprintHandler(store usecases.TrackingStore) *FingerprintHandler {
	return &FingerprintHandler{store: store}
}

// Get looks up the fingerprint for the file identified by the "path" query
// parameter and returns it as JSON.
func (h *FingerprintHandler) Get(c *fiber.Ctx) error {
	path := c.Query("path")
	if path == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path query parameter is required")
	}

	rec, found, err := h.store.Get(c.Context(), path)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if !found {
		return fiber.NewError(fiber.StatusNotFound, "no tracked file for this path")
	}

	return c.JSON(FingerprintResponse{Fingerprint: rec.Fingerprint})
}
