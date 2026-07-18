package v1

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// LibraryEntry is the JSON representation of one scanned file, per the
// music-library-scan capability's GET /api/v1/library contract.
type LibraryEntry struct {
	Path            string  `json:"path"`
	Format          string  `json:"format"`
	DurationSeconds float64 `json:"duration_seconds"`
	Fingerprint     string  `json:"fingerprint"`
	Error           string  `json:"error,omitempty"`
}

// LibraryHandler serves the read-only, in-request local library scan.
type LibraryHandler struct {
	scanner   *usecases.ScanLocalVolume
	musicRoot string
}

func NewLibraryHandler(scanner *usecases.ScanLocalVolume, musicRoot string) *LibraryHandler {
	return &LibraryHandler{scanner: scanner, musicRoot: musicRoot}
}

func (h *LibraryHandler) List(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Minute)
	defer cancel()

	results, err := h.scanner.Scan(ctx, h.musicRoot)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	entries := make([]LibraryEntry, 0, len(results))
	for _, r := range results {
		entries = append(entries, LibraryEntry{
			Path:            r.Path,
			Format:          string(r.Format),
			DurationSeconds: r.Duration.Seconds(),
			Fingerprint:     r.Fingerprint,
			Error:           r.Error,
		})
	}

	return c.JSON(entries)
}
