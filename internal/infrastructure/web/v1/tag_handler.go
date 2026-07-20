package v1

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// TagStatusResponse is the JSON representation of the tag manager's
// current/most recent state, per GET /api/v1/library/tag/status.
type TagStatusResponse struct {
	Running   bool `json:"running"`
	Processed int  `json:"processed"`
	Total     int  `json:"total"`
}

// TagHandler triggers and reports on the background tag-writing job.
type TagHandler struct {
	tag   *usecases.TagManager
	store usecases.TrackingStore
}

func NewTagHandler(tag *usecases.TagManager, store usecases.TrackingStore) *TagHandler {
	return &TagHandler{tag: tag, store: store}
}

// Trigger starts a background tag job over the submitted paths, or over
// every path matching a submitted filter. It returns 202 Accepted if
// started, 400 if no paths/matching filter were submitted, or 409 Conflict
// if a job is already running.
func (h *TagHandler) Trigger(c *fiber.Ctx) error {
	paths, err := resolveSelection(c, h.store)
	if err != nil {
		return err
	}

	if err := h.tag.Start(paths); err != nil {
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
