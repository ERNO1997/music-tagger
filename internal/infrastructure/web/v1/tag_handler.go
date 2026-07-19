package v1

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// TagRequest is the JSON body for POST /api/v1/library/tag.
type TagRequest struct {
	Paths []string `json:"paths"`
}

// TagStatusResponse is the JSON representation of the tag manager's
// current/most recent state, per GET /api/v1/library/tag/status.
type TagStatusResponse struct {
	Running   bool `json:"running"`
	Processed int  `json:"processed"`
	Total     int  `json:"total"`
}

// TagHandler triggers and reports on the background tag-writing job.
type TagHandler struct {
	tag *usecases.TagManager
}

func NewTagHandler(tag *usecases.TagManager) *TagHandler {
	return &TagHandler{tag: tag}
}

// Trigger starts a background tag job over the submitted paths. It
// returns 202 Accepted if started, 400 if no paths were submitted, or 409
// Conflict if a job is already running.
func (h *TagHandler) Trigger(c *fiber.Ctx) error {
	var req TagRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if len(req.Paths) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "paths must not be empty")
	}

	if err := h.tag.Start(req.Paths); err != nil {
		if errors.Is(err, usecases.ErrTagInProgress) {
			return fiber.NewError(fiber.StatusConflict, "a tag job is already in progress")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "accepted"})
}

// Status reports whether a tag job is currently running and its progress.
func (h *TagHandler) Status(c *fiber.Ctx) error {
	status := h.tag.Status()
	return c.JSON(TagStatusResponse{
		Running:   status.Running,
		Processed: status.Processed,
		Total:     status.Total,
	})
}
