package v1

import (
	"github.com/gofiber/fiber/v2"

	"music-tagger/internal/usecases"
)

// AnalyzeStatusResponse is the JSON representation of the background
// analysis manager's current/most recent state, per
// GET /api/v1/library/analyze/status.
type AnalyzeStatusResponse struct {
	Running   bool `json:"running"`
	Processed int  `json:"processed"`
	Total     int  `json:"total"`
}

// AnalyzeHandler reports on the background analysis pass. Unlike
// scan/identify/enrich/tag/relocate, it has no trigger endpoint — the pass
// starts automatically after every refresh completes, never on direct
// request.
type AnalyzeHandler struct {
	analysis *usecases.AnalysisManager
}

func NewAnalyzeHandler(analysis *usecases.AnalysisManager) *AnalyzeHandler {
	return &AnalyzeHandler{analysis: analysis}
}

// Status reports whether an analysis pass is currently running and its
// progress.
func (h *AnalyzeHandler) Status(c *fiber.Ctx) error {
	status := h.analysis.Status()
	return c.JSON(AnalyzeStatusResponse{
		Running:   status.Running,
		Processed: status.Processed,
		Total:     status.Total,
	})
}
