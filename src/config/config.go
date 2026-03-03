package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	NavidromeBase    string
	Port             string
	AdminJWT         string
	MusicLibraryPath string
	YTDLPPath        string
	MetadataProvider string
	Country          string
	Limit            int
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
	var limit = 10
	if l := getEnv("RESULTS_LIMIT", "10"); l != "" {
		if parsed, err := strconv.ParseInt(l, 10, 8); err == nil {
			limit = int(parsed)
		}
	}

	return &Config{
		NavidromeBase:    base,
		Port:             getEnv("PORT", "8080"),
		AdminJWT:         os.Getenv("ADMIN_JWT"),
		MusicLibraryPath: libPath,
		YTDLPPath:        getEnv("YTDLP_PATH", "yt-dlp"),
		MetadataProvider: getEnv("METADATA_PROVIDER", "itunes"),
		Country:          getEnv("COUNTRY", "US"),
		Limit:            limit,
	}, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
