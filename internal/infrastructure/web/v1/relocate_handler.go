package v1

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// RelocateRequest is the JSON body for POST /api/v1/library/relocate.
type RelocateRequest struct {
	Paths []string `json:"paths"`
}

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
}

func NewRelocateHandler(relocate *usecases.RelocateManager) *RelocateHandler {
	return &RelocateHandler{relocate: relocate}
}

// Trigger starts a background relocate job over the submitted paths. It
// returns 202 Accepted if started, 400 if no paths were submitted, or 409
// Conflict if a relocate job or a scan refresh is already running — scan
// and relocate mutually exclude each other (see the RelocateManager doc).
func (h *RelocateHandler) Trigger(c *fiber.Ctx) error {
	var req RelocateRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if len(req.Paths) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "paths must not be empty")
	}

	if err := h.relocate.Start(req.Paths); err != nil {
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
