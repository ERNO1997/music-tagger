package v1

import (
	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/domain"
	"music-tagger/internal/usecases"
)

// audioContentTypes maps a tracked file's format to the Content-Type served
// for its audio bytes — known upfront from the tracked format, no
// per-request sniffing needed.
var audioContentTypes = map[domain.Format]string{
	domain.FormatMP3:  "audio/mpeg",
	domain.FormatFLAC: "audio/flac",
	domain.FormatM4A:  "audio/mp4",
}

// AudioHandler streams a tracked file's own audio bytes for in-browser
// playback.
type AudioHandler struct {
	store usecases.TrackingStore
}

func NewAudioHandler(store usecases.TrackingStore) *AudioHandler {
	return &AudioHandler{store: store}
}

// Serve looks up the file identified by the "path" query parameter and
// streams its audio bytes via c.SendFile (Range-request/seeking support
// comes from fasthttp's own file serving), same trusted-path pattern as
// CoverHandler — the served path always comes from our own tracking store,
// never directly from client input beyond the lookup key. Any tracked,
// non-missing file is playable regardless of identification status;
// playback only needs the file's own bytes.
func (h *AudioHandler) Serve(c *fiber.Ctx) error {
	path := c.Query("path")
	if path == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path query parameter is required")
	}

	rec, found, err := h.store.Get(c.Context(), path)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if !found || rec.Missing {
		return fiber.NewError(fiber.StatusNotFound, "no tracked, non-missing file at this path")
	}

	if err := c.SendFile(rec.Path); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	// Set after SendFile so our known-format Content-Type wins over
	// whatever extension-based type SendFile may have set.
	if contentType, ok := audioContentTypes[rec.Format]; ok {
		c.Set(fiber.HeaderContentType, contentType)
	}
	return nil
}
