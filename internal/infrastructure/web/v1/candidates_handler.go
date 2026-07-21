package v1

import (
	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// CandidateResponse is the JSON representation of one stored identification
// candidate, per GET /api/v1/library/candidates.
type CandidateResponse struct {
	RecordingMBID    string `json:"recording_mbid"`
	Artist           string `json:"artist"`
	Album            string `json:"album"`
	Title            string `json:"title"`
	TrackNumber      int    `json:"track_number,omitempty"`
	AlbumArtist      string `json:"album_artist,omitempty"`
	Year             int    `json:"year,omitempty"`
	DiscNumber       int    `json:"disc_number,omitempty"`
	TotalDiscs       int    `json:"total_discs,omitempty"`
	TotalTracks      int    `json:"total_tracks,omitempty"`
	ReleaseMBID      string `json:"release_mbid,omitempty"`
	ReleaseGroupMBID string `json:"release_group_mbid,omitempty"`
	ArtistMBID       string `json:"artist_mbid,omitempty"`
}

// CandidatesListResponse is the JSON envelope for GET /api/v1/library/candidates.
type CandidatesListResponse struct {
	Candidates []CandidateResponse `json:"candidates"`
}

// CandidatesHandler serves a tracked file's stored identification
// candidates on demand — populated only while its status is `ambiguous`.
type CandidatesHandler struct {
	store usecases.TrackingStore
}

func NewCandidatesHandler(store usecases.TrackingStore) *CandidatesHandler {
	return &CandidatesHandler{store: store}
}

// Get looks up the candidates for the file identified by the "path" query
// parameter and returns them as JSON. 404 for an untracked path; 200 with
// an empty list for a tracked path with no stored candidates.
func (h *CandidatesHandler) Get(c *fiber.Ctx) error {
	path := c.Query("path")
	if path == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path query parameter is required")
	}

	_, found, err := h.store.Get(c.Context(), path)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if !found {
		return fiber.NewError(fiber.StatusNotFound, "no tracked file for this path")
	}

	candidates, err := h.store.GetCandidates(c.Context(), path)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
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
