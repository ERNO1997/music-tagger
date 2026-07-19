package v1

import "github.com/gofiber/fiber/v2"

// RegisterRoutes wires the v1 API surface onto app.
func RegisterRoutes(app *fiber.App, library *LibraryHandler, scan *ScanHandler, identify *IdentifyHandler, enrich *EnrichHandler, cover *CoverHandler, lyrics *LyricsHandler) {
	api := app.Group("/api/v1")
	api.Get("/library", library.List)
	api.Post("/library/scan", scan.Trigger)
	api.Get("/library/scan/status", scan.Status)
	api.Post("/library/identify", identify.Trigger)
	api.Get("/library/identify/status", identify.Status)
	api.Post("/library/enrich", enrich.Trigger)
	api.Get("/library/enrich/status", enrich.Status)
	api.Get("/library/cover", cover.Serve)
	api.Get("/library/lyrics", lyrics.Get)
}
