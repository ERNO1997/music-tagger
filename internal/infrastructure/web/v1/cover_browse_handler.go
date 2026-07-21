package v1

import (
	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// CoverCandidateResponse is the JSON representation of one browsable
// cover-art candidate, per GET /api/v1/library/cover/candidates.
type CoverCandidateResponse struct {
	ReleaseMBID  string `json:"release_mbid"`
	ReleaseTitle string `json:"release_title"`
	ThumbnailURL string `json:"thumbnail_url"`
	ImageURL     string `json:"image_url"`
}

// CoverCandidatesListResponse is the JSON envelope for
// GET /api/v1/library/cover/candidates.
type CoverCandidatesListResponse struct {
	Candidates []CoverCandidateResponse `json:"candidates"`
}

// CoverBrowseHandler serves cover-art candidates across a tracked file's
// release-group's sibling editions, and records a chosen one.
type CoverBrowseHandler struct {
	browse *usecases.BrowseCoverArt
}

func NewCoverBrowseHandler(browse *usecases.BrowseCoverArt) *CoverBrowseHandler {
	return &CoverBrowseHandler{browse: browse}
}

// Candidates returns the cover-art candidates for the file identified by
// the "path" query parameter. 404 for an untracked/unidentified path; 200
// with a (possibly empty) list otherwise.
func (h *CoverBrowseHandler) Candidates(c *fiber.Ctx) error {
	path := c.Query("path")
	if path == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path query parameter is required")
	}

	candidates, found, err := h.browse.Candidates(c.Context(), path)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if !found {
		return fiber.NewError(fiber.StatusNotFound, "no tracked, identified file for this path")
	}

	resp := make([]CoverCandidateResponse, 0, len(candidates))
	for _, cand := range candidates {
		resp = append(resp, CoverCandidateResponse{
			ReleaseMBID:  cand.ReleaseMBID,
			ReleaseTitle: cand.ReleaseTitle,
			ThumbnailURL: cand.ThumbnailURL,
			ImageURL:     cand.ImageURL,
		})
	}

	return c.JSON(CoverCandidatesListResponse{Candidates: resp})
}

// ChooseCoverRequest is POST /api/v1/library/cover/choose's request body.
type ChooseCoverRequest struct {
	Path        string `json:"path"`
	ReleaseMBID string `json:"release_mbid"`
	ImageURL    string `json:"image_url"`
}

// Choose downloads and records a chosen cover-art candidate as the file's
// cover art.
func (h *CoverBrowseHandler) Choose(c *fiber.Ctx) error {
	var req ChooseCoverRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if req.Path == "" || req.ReleaseMBID == "" || req.ImageURL == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path, release_mbid, and image_url are required")
	}

	if err := h.browse.Choose(c.Context(), req.Path, req.ReleaseMBID, req.ImageURL); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(fiber.Map{"status": "chosen"})
}
