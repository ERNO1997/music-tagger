package v1

import "github.com/gofiber/fiber/v2"

// RegisterRoutes wires the v1 API surface onto app.
func RegisterRoutes(app *fiber.App, library *LibraryHandler, scan *ScanHandler) {
	api := app.Group("/api/v1")
	api.Get("/library", library.List)
	api.Post("/library/scan", scan.Trigger)
	api.Get("/library/scan/status", scan.Status)
}
