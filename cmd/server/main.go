package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"

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

	fingerprinter := filestat.NewFpcalcRunner()
	scanner := usecases.NewScanLocalVolume(fingerprinter, store)
	refreshManager := usecases.NewRefreshManager(scanner, musicRoot)

	acoustIDClient := gateways.NewAcoustIDClient(acoustIDKey)
	musicBrainzClient := gateways.NewMusicBrainzClient(musicBrainzUserAgent)
	identifyFile := usecases.NewIdentifyFile(acoustIDClient, musicBrainzClient, store)
	identifyManager := usecases.NewIdentifyManager(identifyFile, store)

	var identifyConfigErr error
	if acoustIDKey == "" || musicBrainzUserAgent == "" {
		identifyConfigErr = fmt.Errorf("identification is not configured: set ACOUSTID_API_KEY and MUSICBRAINZ_USER_AGENT")
	}

	libraryHandler := v1.NewLibraryHandler(store)
	scanHandler := v1.NewScanHandler(refreshManager)
	identifyHandler := v1.NewIdentifyHandler(identifyManager, identifyConfigErr)

	app := fiber.New()

	v1.RegisterRoutes(app, libraryHandler, scanHandler, identifyHandler)

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
