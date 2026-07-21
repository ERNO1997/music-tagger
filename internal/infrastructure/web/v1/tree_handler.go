package v1

import (
	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// TreeDirectoryEntry is one immediate subdirectory under a browsed prefix,
// per GET /api/v1/library/tree.
type TreeDirectoryEntry struct {
	Name            string `json:"name"`
	TotalCount      int    `json:"total_count"`
	IdentifiedCount int    `json:"identified_count"`
}

// TreeFilesEnvelope is the direct-files-at-this-level portion of a tree
// browse response, in the same shape as GET /api/v1/library's envelope.
type TreeFilesEnvelope struct {
	Total   int            `json:"total"`
	Entries []LibraryEntry `json:"entries"`
}

// TreeResponse is the JSON representation of GET /api/v1/library/tree. Path
// echoes back the resolved prefix that was actually browsed (the request's
// own "path" parameter, or the music root when omitted) so a client can
// build subsequent drill-down/breadcrumb requests without needing to know
// the music root's value itself.
type TreeResponse struct {
	Path        string               `json:"path"`
	Directories []TreeDirectoryEntry `json:"directories"`
	Files       TreeFilesEnvelope    `json:"files"`
}

// TreeHandler serves GET /api/v1/library/tree: folder-tree browsing of the
// tracked library, reflecting /music's actual on-disk directory structure.
type TreeHandler struct {
	browse    *usecases.TreeBrowse
	musicRoot string
}

func NewTreeHandler(browse *usecases.TreeBrowse, musicRoot string) *TreeHandler {
	return &TreeHandler{browse: browse, musicRoot: musicRoot}
}

// Get returns the immediate subdirectories and direct files under the
// "path" query parameter (defaulting to the music root), honoring the same
// filter/sort/limit/offset query parameters as GET /api/v1/library.
func (h *TreeHandler) Get(c *fiber.Ctx) error {
	prefix := c.Query("path")
	if prefix == "" {
		prefix = h.musicRoot
	}

	filter, err := parseLibraryFilter(c)
	if err != nil {
		return err
	}
	sortSpec := usecases.LibrarySort{By: c.Query("sort"), Desc: c.Query("order") == "desc"}
	limit, offset, err := parseLibraryPage(c)
	if err != nil {
		return err
	}

	result, err := h.browse.Browse(c.Context(), prefix, filter, sortSpec, limit, offset)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	directories := make([]TreeDirectoryEntry, 0, len(result.Directories))
	for _, d := range result.Directories {
		directories = append(directories, TreeDirectoryEntry{Name: d.Name, TotalCount: d.TotalCount, IdentifiedCount: d.IdentifiedCount})
	}
	entries := make([]LibraryEntry, 0, len(result.Files))
	for _, r := range result.Files {
		entries = append(entries, libraryEntryFrom(r))
	}

	return c.JSON(TreeResponse{
		Path:        prefix,
		Directories: directories,
		Files:       TreeFilesEnvelope{Total: result.FilesTotal, Entries: entries},
	})
}
