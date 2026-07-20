package v1

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

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
	store     usecases.TrackingStore
	configErr error // non-nil when ACOUSTID_API_KEY/MUSICBRAINZ_USER_AGENT is missing
}

func NewIdentifyHandler(identify *usecases.IdentifyManager, store usecases.TrackingStore, configErr error) *IdentifyHandler {
	return &IdentifyHandler{identify: identify, store: store, configErr: configErr}
}

// Trigger starts a background identify job over the submitted paths, or
// over every path matching a submitted filter. It returns 202 Accepted if
// started, 400 if no paths/matching filter were submitted or required
// configuration is missing, or 409 Conflict if a job is already running.
func (h *IdentifyHandler) Trigger(c *fiber.Ctx) error {
	if h.configErr != nil {
		return fiber.NewError(fiber.StatusBadRequest, h.configErr.Error())
	}

	paths, err := resolveSelection(c, h.store)
	if err != nil {
		return err
	}

	if err := h.identify.Start(paths); err != nil {
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
