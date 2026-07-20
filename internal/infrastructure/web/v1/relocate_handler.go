package v1

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// RelocateStatusResponse is the JSON representation of the relocate
// manager's current/most recent state, per GET /api/v1/library/relocate/status.
type RelocateStatusResponse struct {
	Running   bool `json:"running"`
	Processed int  `json:"processed"`
	Total     int  `json:"total"`
}

// RelocateHandler triggers and reports on the background relocate job.
type RelocateHandler struct {
	relocate *usecases.RelocateManager
	store    usecases.TrackingStore
}

func NewRelocateHandler(relocate *usecases.RelocateManager, store usecases.TrackingStore) *RelocateHandler {
	return &RelocateHandler{relocate: relocate, store: store}
}

// Trigger starts a background relocate job over the submitted paths, or
// over every path matching a submitted filter. It returns 202 Accepted if
// started, 400 if no paths/matching filter were submitted, or 409 Conflict
// if a relocate job or a scan refresh is already running — scan and
// relocate mutually exclude each other (see the RelocateManager doc).
func (h *RelocateHandler) Trigger(c *fiber.Ctx) error {
	paths, err := resolveSelection(c, h.store)
	if err != nil {
		return err
	}

	if err := h.relocate.Start(paths); err != nil {
		if errors.Is(err, usecases.ErrRelocateInProgress) {
			return fiber.NewError(fiber.StatusConflict, "a relocate job is already in progress")
		}
		if errors.Is(err, usecases.ErrBlockedByScan) {
			return fiber.NewError(fiber.StatusConflict, "a scan refresh is in progress")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "accepted"})
}

// Status reports whether a relocate job is currently running and its progress.
func (h *RelocateHandler) Status(c *fiber.Ctx) error {
	status := h.relocate.Status()
	return c.JSON(RelocateStatusResponse{
		Running:   status.Running,
		Processed: status.Processed,
		Total:     status.Total,
	})
}
