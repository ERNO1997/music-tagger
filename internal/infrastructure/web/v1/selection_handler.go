package v1

import (
	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// SelectionHandler serves a paginated read of the current selection — the
// same {paths, filter} request shape POST /api/v1/library/identify etc.
// already accept to trigger a job, but returning matching entries instead
// of starting one.
type SelectionHandler struct {
	store usecases.TrackingStore
}

func NewSelectionHandler(store usecases.TrackingStore) *SelectionHandler {
	return &SelectionHandler{store: store}
}

// List returns a page of tracked entries matching the request body's
// explicit path list or filter, honoring the same sort/order/limit/offset
// query parameters and LibraryListResponse shape as GET /api/v1/library.
func (h *SelectionHandler) List(c *fiber.Ctx) error {
	filter, err := selectionFilter(c)
	if err != nil {
		return err
	}

	sort := usecases.LibrarySort{
		By:   c.Query("sort"),
		Desc: c.Query("order") == "desc",
	}

	limit, offset, err := parseLibraryPage(c)
	if err != nil {
		return err
	}

	records, total, err := h.store.QueryPage(c.Context(), filter, sort, limit, offset)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	entries := make([]LibraryEntry, 0, len(records))
	for _, r := range records {
		entries = append(entries, libraryEntryFrom(r))
	}

	return c.JSON(LibraryListResponse{Total: total, Entries: entries})
}
