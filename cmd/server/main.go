package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"

	"music-tagger/internal/infrastructure/covers"
	"music-tagger/internal/infrastructure/filestat"
	"music-tagger/internal/infrastructure/gateways"
	"music-tagger/internal/infrastructure/persistence"
	v1 "music-tagger/internal/infrastructure/web/v1"
	"music-tagger/internal/usecases"
	"music-tagger/ui"
)

func main() {
	musicRoot := os.Getenv("MUSIC_DIR")
	if musicRoot == "" {
		musicRoot = "/music"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "/data/music-tagger.db"
	}

	acoustIDKey := os.Getenv("ACOUSTID_API_KEY")
	musicBrainzUserAgent := os.Getenv("MUSICBRAINZ_USER_AGENT")

	store, err := persistence.NewSQLiteStore(context.Background(), dbPath)
	if err != nil {
		log.Fatalf("opening tracking store: %v", err)
	}
	defer store.Close()

	durationReader := filestat.NewTagLibDurationReader()
	rawTagReader := filestat.NewTagLibRawTagReader()
	scanner := usecases.NewScanLocalVolume(durationReader, rawTagReader, store)
	refreshManager := usecases.NewRefreshManager(scanner, musicRoot)

	fingerprinter := filestat.NewFpcalcRunner()
	acoustIDClient := gateways.NewAcoustIDClient(acoustIDKey)
	musicBrainzClient := gateways.NewMusicBrainzClient(musicBrainzUserAgent)
	identifyFile := usecases.NewIdentifyFile(acoustIDClient, musicBrainzClient, fingerprinter, store)
	identifyManager := usecases.NewIdentifyManager(identifyFile)
	manualSearch := usecases.NewManualSearch(musicBrainzClient, store)

	var identifyConfigErr error
	if acoustIDKey == "" || musicBrainzUserAgent == "" {
		identifyConfigErr = fmt.Errorf("identification is not configured: set ACOUSTID_API_KEY and MUSICBRAINZ_USER_AGENT")
	}

	coverArtStore, err := covers.NewStore(filepath.Dir(dbPath))
	if err != nil {
		log.Fatalf("setting up cover art storage: %v", err)
	}
	coverArtClient := gateways.NewCoverArtClient(musicBrainzUserAgent)
	lrclibClient := gateways.NewLRCLIBClient(musicBrainzUserAgent)
	enrichFile := usecases.NewEnrichFile(coverArtClient, coverArtStore, lrclibClient, store)
	enrichManager := usecases.NewEnrichManager(enrichFile, store)
	browseCoverArt := usecases.NewBrowseCoverArt(musicBrainzClient, coverArtClient, coverArtStore, store)

	tagger := filestat.NewTagLibTagger()
	tagFile := usecases.NewTagFile(tagger, store)
	tagManager := usecases.NewTagManager(tagFile)

	relocator := filestat.NewPathRelocator(musicRoot)
	relocateFile := usecases.NewRelocateFile(relocator, store)
	relocateManager := usecases.NewRelocateManager(relocateFile, refreshManager)
	refreshManager.SetRelocateStatus(relocateManager)

	analysisManager := usecases.NewAnalysisManager(fingerprinter, tagger, coverArtStore, store, relocator)
	analysisManager.SetRelocateStatus(relocateManager)
	refreshManager.SetAnalysisManager(analysisManager)
	relocateManager.SetAnalysisStatus(analysisManager)

	deleteMissingFile := usecases.NewDeleteMissingFile(store)

	libraryHandler := v1.NewLibraryHandler(store)
	selectionHandler := v1.NewSelectionHandler(store)
	scanHandler := v1.NewScanHandler(refreshManager)
	identifyHandler := v1.NewIdentifyHandler(identifyManager, manualSearch, store, identifyConfigErr)
	enrichHandler := v1.NewEnrichHandler(enrichManager, store)
	coverHandler := v1.NewCoverHandler(store)
	lyricsHandler := v1.NewLyricsHandler(store)
	tagHandler := v1.NewTagHandler(tagManager, store)
	embeddedTagsHandler := v1.NewEmbeddedTagsHandler(tagFile)
	relocateHandler := v1.NewRelocateHandler(relocateManager, store)
	fingerprintHandler := v1.NewFingerprintHandler(store)
	candidatesHandler := v1.NewCandidatesHandler(store)
	coverBrowseHandler := v1.NewCoverBrowseHandler(browseCoverArt)
	deleteHandler := v1.NewDeleteHandler(deleteMissingFile)

	treeBrowse := usecases.NewTreeBrowse(store)
	treeHandler := v1.NewTreeHandler(treeBrowse, musicRoot)
	completenessChecker := usecases.NewCompletenessChecker(store, musicBrainzClient)
	artistAlbumHandler := v1.NewArtistAlbumHandler(store, completenessChecker)
	audioHandler := v1.NewAudioHandler(store)
	analyzeHandler := v1.NewAnalyzeHandler(analysisManager)

	app := fiber.New()

	v1.RegisterRoutes(app, libraryHandler, scanHandler, identifyHandler, enrichHandler, coverHandler, lyricsHandler, tagHandler, embeddedTagsHandler, relocateHandler, fingerprintHandler, candidatesHandler, coverBrowseHandler, deleteHandler, treeHandler, artistAlbumHandler, audioHandler, selectionHandler, analyzeHandler)

	app.Use("/", filesystem.New(filesystem.Config{
		Root: http.FS(ui.Assets),
	}))

	if err := refreshManager.Start(); err != nil {
		log.Printf("startup refresh not started: %v", err)
	}

	log.Printf("music-tagger listening on :%s (music dir: %s, db: %s)", port, musicRoot, dbPath)
	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}
}
