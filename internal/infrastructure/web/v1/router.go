package v1

import "github.com/gofiber/fiber/v2"

// RegisterRoutes wires the v1 API surface onto app.
func RegisterRoutes(app *fiber.App, library *LibraryHandler, scan *ScanHandler, identify *IdentifyHandler, enrich *EnrichHandler, cover *CoverHandler, lyrics *LyricsHandler, tag *TagHandler, embeddedTags *EmbeddedTagsHandler, relocate *RelocateHandler, fingerprint *FingerprintHandler, candidates *CandidatesHandler, coverBrowse *CoverBrowseHandler, del *DeleteHandler, tree *TreeHandler, artistAlbum *ArtistAlbumHandler, audio *AudioHandler) {
	api := app.Group("/api/v1")
	api.Get("/library", library.List)
	api.Post("/library/scan", scan.Trigger)
	api.Get("/library/scan/status", scan.Status)
	api.Post("/library/identify", identify.Trigger)
	api.Get("/library/identify/status", identify.Status)
	api.Post("/library/identify/resolve", identify.Resolve)
	api.Post("/library/identify/search", identify.Search)
	api.Post("/library/enrich", enrich.Trigger)
	api.Get("/library/enrich/status", enrich.Status)
	api.Get("/library/cover", cover.Serve)
	api.Get("/library/cover/candidates", coverBrowse.Candidates)
	api.Post("/library/cover/choose", coverBrowse.Choose)
	api.Get("/library/lyrics", lyrics.Get)
	api.Post("/library/tag", tag.Trigger)
	api.Get("/library/tag/status", tag.Status)
	api.Get("/library/tags", embeddedTags.Get)
	api.Post("/library/relocate", relocate.Trigger)
	api.Get("/library/relocate/status", relocate.Status)
	api.Get("/library/fingerprint", fingerprint.Get)
	api.Get("/library/candidates", candidates.Get)
	api.Delete("/library/entry", del.Delete)
	api.Get("/library/tree", tree.Get)
	api.Get("/library/artists", artistAlbum.Artists)
	api.Get("/library/albums", artistAlbum.Albums)
	api.Get("/library/tracks", artistAlbum.Tracks)
	api.Get("/library/audio", audio.Serve)
}
