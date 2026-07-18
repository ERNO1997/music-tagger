package v1

import (
	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// LibraryEntry is the JSON representation of one tracked file, per the
// music-library-scan capability's GET /api/v1/library contract.
type LibraryEntry struct {
	Path            string  `json:"path"`
	Format          string  `json:"format"`
	DurationSeconds float64 `json:"duration_seconds"`
	Fingerprint     string  `json:"fingerprint"`
	Status          string  `json:"status"`
	Error           string  `json:"error,omitempty"`
}

// LibraryHandler serves the current tracked state read directly from the
// TrackingStore — no disk walk or fingerprinting happens on this path.
type LibraryHandler struct {
	store usecases.TrackingStore
}

func NewLibraryHandler(store usecases.TrackingStore) *LibraryHandler {
	return &LibraryHandler{store: store}
}

func (h *LibraryHandler) List(c *fiber.Ctx) error {
	records, err := h.store.LoadAll(c.Context())
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	entries := make([]LibraryEntry, 0, len(records))
	for _, r := range records {
		entries = append(entries, LibraryEntry{
			Path:            r.Path,
			Format:          string(r.Format),
			DurationSeconds: r.DurationSeconds,
			Fingerprint:     r.Fingerprint,
			Status:          string(r.EffectiveStatus()),
			Error:           r.FingerprintError,
		})
	}

	return c.JSON(entries)
}
