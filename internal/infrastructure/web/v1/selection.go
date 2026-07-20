package v1

import (
	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// SelectionFilter is the wire shape of a LibraryFilter inside a trigger
// request body.
type SelectionFilter struct {
	Status    string `json:"status"`
	Tagged    *bool  `json:"tagged"`
	Relocated *bool  `json:"relocated"`
	Search    string `json:"q"`
}

// SelectionRequest is a trigger endpoint's request body: either an explicit
// path list (unchanged, for page-sized selections) or a filter to resolve
// into a path list at execution time (for "select all N matching").
type SelectionRequest struct {
	Paths  []string         `json:"paths"`
	Filter *SelectionFilter `json:"filter"`
}

// resolveSelection parses c's body as a SelectionRequest and returns the
// concrete path list to act on: req.Paths verbatim if non-empty, otherwise
// every path currently matching req.Filter (resolved via store.QueryPaths).
// Returns a 400 fiber.Error if the body is invalid or neither paths nor a
// filter yields anything to act on.
func resolveSelection(c *fiber.Ctx, store usecases.TrackingStore) ([]string, error) {
	var req SelectionRequest
	if err := c.BodyParser(&req); err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	if len(req.Paths) > 0 {
		return req.Paths, nil
	}

	if req.Filter == nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "paths must not be empty")
	}

	filter := usecases.LibraryFilter{
		Status:    req.Filter.Status,
		Tagged:    req.Filter.Tagged,
		Relocated: req.Filter.Relocated,
		Search:    req.Filter.Search,
	}
	paths, err := store.QueryPaths(c.Context(), filter)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if len(paths) == 0 {
		return nil, fiber.NewError(fiber.StatusBadRequest, "no files match the given filter")
	}
	return paths, nil
}
