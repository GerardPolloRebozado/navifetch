package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	NavidromeBase    string
	Port             string
	AdminJWT         string
	MusicLibraryPath string
	YTDLPPath        string
}

func LoadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error loading it")
	}

	base := os.Getenv("NAVIDROME_BASE")
	if base == "" {
		log.Fatal("NAVIDROME_BASE env var is required (e.g., http://localhost:4533)")
	}

	libPath := getEnv("MUSIC_LIBRARY_PATH", "/music")

	return &Config{
		NavidromeBase:    base,
		Port:             getEnv("PORT", "8080"),
		AdminJWT:         os.Getenv("ADMIN_JWT"),
		MusicLibraryPath: libPath,
		YTDLPPath:        getEnv("YTDLP_PATH", "yt-dlp"),
	}, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
