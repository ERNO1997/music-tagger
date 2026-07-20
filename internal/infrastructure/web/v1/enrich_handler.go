package v1

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// EnrichStatusResponse is the JSON representation of the enrich manager's
// current/most recent state, per GET /api/v1/library/enrich/status.
type EnrichStatusResponse struct {
	Running   bool `json:"running"`
	Processed int  `json:"processed"`
	Total     int  `json:"total"`
}

// EnrichHandler triggers and reports on the background cover art enrich job.
type EnrichHandler struct {
	enrich *usecases.EnrichManager
	store  usecases.TrackingStore
}

func NewEnrichHandler(enrich *usecases.EnrichManager, store usecases.TrackingStore) *EnrichHandler {
	return &EnrichHandler{enrich: enrich, store: store}
}

// Trigger starts a background enrich job over the submitted paths, or over
// every path matching a submitted filter. It returns 202 Accepted if
// started, 400 if no paths/matching filter were submitted, or 409 Conflict
// if a job is already running. Cover Art Archive needs no API key, so there
// is no configuration-error case here unlike identify.
func (h *EnrichHandler) Trigger(c *fiber.Ctx) error {
	paths, err := resolveSelection(c, h.store)
	if err != nil {
		return err
	}

	if err := h.enrich.Start(paths); err != nil {
		if errors.Is(err, usecases.ErrEnrichInProgress) {
			return fiber.NewError(fiber.StatusConflict, "an enrich job is already in progress")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "accepted"})
}

// Status reports whether an enrich job is currently running and its progress.
func (h *EnrichHandler) Status(c *fiber.Ctx) error {
	status := h.enrich.Status()
	return c.JSON(EnrichStatusResponse{
		Running:   status.Running,
		Processed: status.Processed,
		Total:     status.Total,
	})
}
