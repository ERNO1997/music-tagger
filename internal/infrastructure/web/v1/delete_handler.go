package v1

import (
	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// DeleteHandler removes a tracked file's row, gated to files confirmed
// missing from disk.
type DeleteHandler struct {
	deleteMissing *usecases.DeleteMissingFile
}

func NewDeleteHandler(deleteMissing *usecases.DeleteMissingFile) *DeleteHandler {
	return &DeleteHandler{deleteMissing: deleteMissing}
}

// Delete removes the tracked record identified by the "path" query
// parameter. Returns 204 on success, 404 if untracked, or 409 Conflict if
// the record's status isn't missing.
func (h *DeleteHandler) Delete(c *fiber.Ctx) error {
	path := c.Query("path")
	if path == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path query parameter is required")
	}

	outcome, err := h.deleteMissing.Delete(c.Context(), path)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	switch outcome {
	case usecases.DeleteOutcomeNotFound:
		return fiber.NewError(fiber.StatusNotFound, "no tracked file for this path")
	case usecases.DeleteOutcomeNotMissing:
		return fiber.NewError(fiber.StatusConflict, "file is not marked missing")
	default:
		return c.SendStatus(fiber.StatusNoContent)
	}
}
