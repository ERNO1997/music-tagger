package v1

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// IdentifyStatusResponse is the JSON representation of the identify
// manager's current/most recent state, per GET /api/v1/library/identify/status.
type IdentifyStatusResponse struct {
	Running   bool `json:"running"`
	Processed int  `json:"processed"`
	Total     int  `json:"total"`
}

// IdentifyHandler triggers and reports on the background identify job.
type IdentifyHandler struct {
	identify     *usecases.IdentifyManager
	manualSearch *usecases.ManualSearch
	store        usecases.TrackingStore
	configErr    error // non-nil when ACOUSTID_API_KEY/MUSICBRAINZ_USER_AGENT is missing
}

func NewIdentifyHandler(identify *usecases.IdentifyManager, manualSearch *usecases.ManualSearch, store usecases.TrackingStore, configErr error) *IdentifyHandler {
	return &IdentifyHandler{identify: identify, manualSearch: manualSearch, store: store, configErr: configErr}
}

// Trigger starts a background identify job over the submitted paths, or
// over every path matching a submitted filter. It returns 202 Accepted if
// started, 400 if no paths/matching filter were submitted or required
// configuration is missing, or 409 Conflict if a job is already running.
func (h *IdentifyHandler) Trigger(c *fiber.Ctx) error {
	if h.configErr != nil {
		return fiber.NewError(fiber.StatusBadRequest, h.configErr.Error())
	}

	paths, err := resolveSelection(c, h.store)
	if err != nil {
		return err
	}

	if err := h.identify.Start(paths); err != nil {
		if errors.Is(err, usecases.ErrIdentifyInProgress) {
			return fiber.NewError(fiber.StatusConflict, "an identify job is already in progress")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "accepted"})
}

// Status reports whether an identify job is currently running and its progress.
func (h *IdentifyHandler) Status(c *fiber.Ctx) error {
	status := h.identify.Status()
	return c.JSON(IdentifyStatusResponse{
		Running:   status.Running,
		Processed: status.Processed,
		Total:     status.Total,
	})
}

// ResolveRequest is POST /api/v1/library/identify/resolve's request body.
type ResolveRequest struct {
	Path          string `json:"path"`
	RecordingMBID string `json:"recording_mbid"`
}

// Resolve records a stored candidate as an ambiguous file's resolved
// identification. Unlike Trigger, this responds synchronously — resolving
// an already-computed candidate requires no external network call.
func (h *IdentifyHandler) Resolve(c *fiber.Ctx) error {
	var req ResolveRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if req.Path == "" || req.RecordingMBID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path and recording_mbid are required")
	}

	found, err := h.identify.ResolveAmbiguous(c.Context(), req.Path, req.RecordingMBID)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if !found {
		return fiber.NewError(fiber.StatusNotFound, "no matching candidate for this path")
	}

	return c.JSON(fiber.Map{"status": "resolved"})
}

// SearchRequest is POST /api/v1/library/identify/search's request body.
type SearchRequest struct {
	Path  string `json:"path"`
	Query string `json:"query"`
}

// Search performs a manual, free-text MusicBrainz search for a tracked
// file — independent of any audio fingerprint — and records the results
// as its candidates, responding synchronously with the same shape
// GET /api/v1/library/candidates already returns.
func (h *IdentifyHandler) Search(c *fiber.Ctx) error {
	var req SearchRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if req.Path == "" || req.Query == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path and query are required")
	}

	candidates, found, err := h.manualSearch.Search(c.Context(), req.Path, req.Query)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if !found {
		return fiber.NewError(fiber.StatusNotFound, "no tracked file for this path")
	}

	resp := make([]CandidateResponse, 0, len(candidates))
	for _, cand := range candidates {
		resp = append(resp, CandidateResponse{
			RecordingMBID:    cand.RecordingID,
			Artist:           cand.Artist,
			Album:            cand.Album,
			Title:            cand.Title,
			TrackNumber:      cand.TrackNumber,
			AlbumArtist:      cand.AlbumArtist,
			Year:             cand.Year,
			DiscNumber:       cand.DiscNumber,
			TotalDiscs:       cand.TotalDiscs,
			TotalTracks:      cand.TotalTracks,
			ReleaseMBID:      cand.ReleaseMBID,
			ReleaseGroupMBID: cand.ReleaseGroupMBID,
			ArtistMBID:       cand.ArtistMBID,
		})
	}

	return c.JSON(CandidatesListResponse{Candidates: resp})
}
