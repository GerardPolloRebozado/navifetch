package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	NavidromeBase    string
	Port             string
	AdminJWT         string
	MusicLibraryPath string
	YTDLPPath        string
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Expose-Headers", "X-Content-Duration, X-Total-Count, X-Nd-Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
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

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s%s from %s", r.Method, r.Host, r.URL.RequestURI(), r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	rp, err := NewReverseProxy(cfg.NavidromeBase)
	if err != nil {
		log.Fatalf("proxy creation error: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/rest/search3.view", SmartSearch(cfg.NavidromeBase))
	mux.HandleFunc("/rest/search2.view", SmartSearch(cfg.NavidromeBase))
	mux.HandleFunc("/rest/search3", SmartSearch(cfg.NavidromeBase))
	mux.HandleFunc("/rest/search2", SmartSearch(cfg.NavidromeBase))

	mux.HandleFunc("/rest/getCoverArt.view", ProxyCoverArt(rp))
	mux.HandleFunc("/rest/getCoverArt", ProxyCoverArt(rp))

	mux.HandleFunc("/rest/stream.view", ProxyStream(cfg, rp, false))
	mux.HandleFunc("/rest/stream", ProxyStream(cfg, rp, false))
	mux.HandleFunc("/rest/download.view", ProxyStream(cfg, rp, true))
	mux.HandleFunc("/rest/download", ProxyStream(cfg, rp, true))

	mux.HandleFunc("/rest/getSong.view", ProxyMetadata(rp))
	mux.HandleFunc("/rest/getSong", ProxyMetadata(rp))

	mux.HandleFunc("/rest/createPlaylist.view", ProxyPlaylistOrQueue(cfg, rp, true))
	mux.HandleFunc("/rest/createPlaylist", ProxyPlaylistOrQueue(cfg, rp, true))
	mux.HandleFunc("/rest/updatePlaylist.view", ProxyPlaylistOrQueue(cfg, rp, true))
	mux.HandleFunc("/rest/updatePlaylist", ProxyPlaylistOrQueue(cfg, rp, true))
	mux.HandleFunc("/rest/savePlayQueue.view", ProxyPlaylistOrQueue(cfg, rp, false))
	mux.HandleFunc("/rest/savePlayQueue", ProxyPlaylistOrQueue(cfg, rp, false))

	// Catch-all reverse proxy
	mux.Handle("/", rp)

	// Middleware
	handler := LoggingMiddleware(mux)
	handler = CORSMiddleware(handler)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start background services
	StartCleanupCron(cfg)

	log.Printf("Proxy listening on :%s, forwarding to %s", cfg.Port, cfg.NavidromeBase)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
