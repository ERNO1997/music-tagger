package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"

	"music-tagger/internal/infrastructure/filestat"
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

	fingerprinter := filestat.NewFpcalcRunner()
	scanner := usecases.NewScanLocalVolume(fingerprinter)
	libraryHandler := v1.NewLibraryHandler(scanner, musicRoot)

	app := fiber.New()

	v1.RegisterRoutes(app, libraryHandler)

	app.Use("/", filesystem.New(filesystem.Config{
		Root: http.FS(ui.Assets),
	}))

	log.Printf("music-tagger listening on :%s (music dir: %s)", port, musicRoot)
	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}
}
