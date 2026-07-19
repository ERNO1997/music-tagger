package v1

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// ScanStatusResponse is the JSON representation of the refresh manager's
// current/most recent state, per GET /api/v1/library/scan/status.
type ScanStatusResponse struct {
	Running   bool `json:"running"`
	Processed int  `json:"processed"`
	Total     int  `json:"total"`
}

// ScanHandler triggers and reports on the background library refresh.
type ScanHandler struct {
	refresh *usecases.RefreshManager
}

func NewScanHandler(refresh *usecases.RefreshManager) *ScanHandler {
	return &ScanHandler{refresh: refresh}
}

// Trigger starts a background refresh. It returns 202 Accepted if started,
// or 409 Conflict if one is already running, or if a relocate job is
// running — scan and relocate mutually exclude each other (see
// RefreshManager.SetRelocateStatus).
func (h *ScanHandler) Trigger(c *fiber.Ctx) error {
	if err := h.refresh.Start(); err != nil {
		if errors.Is(err, usecases.ErrRefreshInProgress) {
			return fiber.NewError(fiber.StatusConflict, "a refresh is already in progress")
		}
		if errors.Is(err, usecases.ErrBlockedByRelocate) {
			return fiber.NewError(fiber.StatusConflict, "a relocate job is in progress")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "accepted"})
}

// Status reports whether a refresh is currently running and its progress.
func (h *ScanHandler) Status(c *fiber.Ctx) error {
	status := h.refresh.Status()
	return c.JSON(ScanStatusResponse{
		Running:   status.Running,
		Processed: status.Processed,
		Total:     status.Total,
	})
}
