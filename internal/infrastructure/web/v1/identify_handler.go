package v1

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// IdentifyRequest is the JSON body for POST /api/v1/library/identify.
type IdentifyRequest struct {
	Paths []string `json:"paths"`
}

// IdentifyStatusResponse is the JSON representation of the identify
// manager's current/most recent state, per GET /api/v1/library/identify/status.
type IdentifyStatusResponse struct {
	Running   bool `json:"running"`
	Processed int  `json:"processed"`
	Total     int  `json:"total"`
}

// IdentifyHandler triggers and reports on the background identify job.
type IdentifyHandler struct {
	identify  *usecases.IdentifyManager
	configErr error // non-nil when ACOUSTID_API_KEY/MUSICBRAINZ_USER_AGENT is missing
}

func NewIdentifyHandler(identify *usecases.IdentifyManager, configErr error) *IdentifyHandler {
	return &IdentifyHandler{identify: identify, configErr: configErr}
}

// Trigger starts a background identify job over the submitted paths. It
// returns 202 Accepted if started, 400 if no paths were submitted or
// required configuration is missing, or 409 Conflict if a job is already
// running.
func (h *IdentifyHandler) Trigger(c *fiber.Ctx) error {
	if h.configErr != nil {
		return fiber.NewError(fiber.StatusBadRequest, h.configErr.Error())
	}

	var req IdentifyRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if len(req.Paths) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "paths must not be empty")
	}

	if err := h.identify.Start(req.Paths); err != nil {
		if errors.Is(err, usecases.ErrIdentifyInProgress) {
			return fiber.NewError(fiber.StatusConflict, "an identify job is already in progress")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "accepted"})
}

// Status reports whether an identify job is currently running and its progress.
func (h *IdentifyHandler) Status(c *fiber.Ctx) error {
	status := h.identify.Status()
	return c.JSON(IdentifyStatusResponse{
		Running:   status.Running,
		Processed: status.Processed,
		Total:     status.Total,
	})
}
