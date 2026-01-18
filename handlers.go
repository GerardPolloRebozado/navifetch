package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ProxyMetadata handles the getSong request for external iTunes items
func ProxyMetadata(rp http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")

		if strings.HasPrefix(id, "itunes-") {
			trackID := strings.TrimPrefix(id, "itunes-")

			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			results, err := ITunesLookup(ctx, trackID)
			if err != nil || len(results) == 0 {
				http.Error(w, "Song not found", http.StatusNotFound)
				return
			}
			track := results[0]

			// Map to Subsonic Song
			song := map[string]any{
				"id":          "itunes-" + trackID,
				"title":       track.TrackName,
				"artist":      track.ArtistName,
				"album":       track.CollectionName,
				"coverArt":    "itunes-cover-" + track.ArtworkUrl100,
				"duration":    track.TrackTimeMillis / 1000,
				"isDir":       false,
				"isVideo":     false,
				"suffix":      "mp3",
				"contentType": "audio/mpeg",
				"path":        "virtual/itunes/" + trackID,
			}

			resp := map[string]any{
				"subsonic-response": map[string]any{
					"status":  "ok",
					"version": "1.16.1",
					"song":    song,
				},
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(resp)
			return
		}

		rp.ServeHTTP(w, r)
	}
}

// ProxyStream handles streaming and downloading
func ProxyStream(cfg *Config, rp http.Handler, permanent bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")

		if strings.HasPrefix(id, "itunes-") {
			trackID := strings.TrimPrefix(id, "itunes-")

			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			results, err := ITunesLookup(ctx, trackID)
			if err != nil || len(results) == 0 {
				log.Printf("iTunes lookup failed for ID %s: %v", trackID, err)
				http.Error(w, "Track not found", http.StatusNotFound)
				return
			}
			track := results[0]

			fullPath := GetTrackPath(cfg, track.ArtistName, track.CollectionName, track.TrackName, permanent)
			_ = os.MkdirAll(filepath.Dir(fullPath), 0755)

			// Check if exists
			if _, err := os.Stat(fullPath); err == nil {
				log.Printf("Serving existing file: %s", fullPath)
				http.ServeFile(w, r, fullPath)
				return
			}

			// Download and Stream
			log.Printf("Downloading and streaming: %s - %s", track.ArtistName, track.TrackName)
			searchQuery := fmt.Sprintf("ytsearch1:%s - %s Audio", track.ArtistName, track.TrackName)

			cmd := exec.CommandContext(r.Context(), cfg.YTDLPPath,
				"-x", "--audio-format", "mp3",
				"-o", "-",
				"--no-playlist",
				searchQuery,
			)

			w.Header().Set("Content-Type", "audio/mpeg")
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				http.Error(w, "Stream error", http.StatusInternalServerError)
				return
			}

			if err := cmd.Start(); err != nil {
				log.Printf("yt-dlp start error: %v", err)
				http.Error(w, "Failed to start download", http.StatusInternalServerError)
				return
			}

			_, _ = io.Copy(w, stdout)
			_ = cmd.Wait()

			go func() {
				// Pass high-res cover URL and Auth Params for scanning
				coverURL := GetHighResArtwork(track.ArtworkUrl100)
				download(cfg.YTDLPPath, cfg.NavidromeBase, track.ArtistName, track.CollectionName, track.TrackName, fullPath, coverURL, r.URL.Query())
			}()

			return
		}

		rp.ServeHTTP(w, r)
	}
}

// ProxyPlaylistOrQueue handles background downloads for playlist actions
func ProxyPlaylistOrQueue(cfg *Config, rp http.Handler, permanent bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids := []string{}
		ids = append(ids, r.URL.Query()["songId"]...)
		ids = append(ids, r.URL.Query()["songIdToAdd"]...)
		ids = append(ids, r.URL.Query()["id"]...)

		hasItunes := false
		authParams := r.URL.Query()
		for _, id := range ids {
			if strings.HasPrefix(id, "itunes-") {
				hasItunes = true
				go triggerDownload(cfg, id, authParams, permanent)
			}
		}

		if hasItunes {
			resp := map[string]any{
				"subsonic-response": map[string]any{
					"status":  "ok",
					"version": "1.16.1",
				},
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			json.NewEncoder(w).Encode(resp)
			return
		}

		rp.ServeHTTP(w, r)
	}
}

func triggerDownload(cfg *Config, fullID string, authParams url.Values, permanent bool) {
	trackID := strings.TrimPrefix(fullID, "itunes-")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := ITunesLookup(ctx, trackID)
	if err != nil || len(results) == 0 {
		log.Printf("Background download failed lookup for %s: %v", trackID, err)
		return
	}
	track := results[0]

	fullPath := GetTrackPath(cfg, track.ArtistName, track.CollectionName, track.TrackName, permanent)
	coverURL := GetHighResArtwork(track.ArtworkUrl100)

	download(cfg.YTDLPPath, cfg.NavidromeBase, track.ArtistName, track.CollectionName, track.TrackName, fullPath, coverURL, authParams)
}
