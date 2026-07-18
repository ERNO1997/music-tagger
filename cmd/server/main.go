package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"

	"music-tagger/internal/infrastructure/filestat"
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

	store, err := persistence.NewSQLiteStore(context.Background(), dbPath)
	if err != nil {
		log.Fatalf("opening tracking store: %v", err)
	}
	defer store.Close()

	fingerprinter := filestat.NewFpcalcRunner()
	scanner := usecases.NewScanLocalVolume(fingerprinter, store)
	refreshManager := usecases.NewRefreshManager(scanner, musicRoot)

	libraryHandler := v1.NewLibraryHandler(store)
	scanHandler := v1.NewScanHandler(refreshManager)

	app := fiber.New()

	v1.RegisterRoutes(app, libraryHandler, scanHandler)

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
