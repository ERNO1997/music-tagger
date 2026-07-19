package v1

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// EnrichRequest is the JSON body for POST /api/v1/library/enrich.
type EnrichRequest struct {
	Paths []string `json:"paths"`
}

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
}

func NewEnrichHandler(enrich *usecases.EnrichManager) *EnrichHandler {
	return &EnrichHandler{enrich: enrich}
}

// Trigger starts a background enrich job over the submitted paths. It
// returns 202 Accepted if started, 400 if no paths were submitted, or 409
// Conflict if a job is already running. Cover Art Archive needs no API
// key, so there is no configuration-error case here unlike identify.
func (h *EnrichHandler) Trigger(c *fiber.Ctx) error {
	var req EnrichRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if len(req.Paths) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "paths must not be empty")
	}

	if err := h.enrich.Start(req.Paths); err != nil {
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
