package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/GerardPolloRebozado/navifetch/src/config"
	"github.com/GerardPolloRebozado/navifetch/src/metadata"
	"github.com/GerardPolloRebozado/navifetch/src/service"
)

type Handler struct {
	cfg           *config.Config
	rp            *service.SubsonicReverseProxy
	metadata      metadata.Provider
	albumService  *service.AlbumService
	searchService *service.SearchService
	songService   *service.SongService
	streamService *service.StreamService
}

func NewHandler(cfg *config.Config, rp *service.SubsonicReverseProxy) *Handler {
	p, err := metadata.NewProvider(cfg.MetadataProvider, cfg.Country, cfg.Limit, cfg.LastFMApiKey)
	if err != nil {
		log.Fatalf("Failed to initialize metadata provider: %v", err)
	}
	return &Handler{
		cfg:           cfg,
		rp:            rp,
		metadata:      p,
		albumService:  service.NewAlbumService(rp, p),
		searchService: service.NewSearchService(rp, p),
		songService:   service.NewSongService(rp, p),
		streamService: service.NewStreamService(cfg, p),
	}
}

func (h *Handler) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) SmartSearch(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	if query == "" {
		query = r.URL.Query().Get("q")
	}
	if query == "" {
		query = r.URL.Query().Get("any")
	}
	if query == "" {
		query = r.URL.Query().Get("song")
	}
	if query == "" {
		query = r.URL.Query().Get("title")
	}
	if query == "" {
		query = r.URL.Query().Get("artist")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	log.Printf("Search query: %s", query)
	if query == "\"\"" || query == "" {
		h.rp.ServeHTTP(w, r)
		return
	}
	body, contentType, err := h.searchService.SmartSearch(ctx, query, r.URL.Path, r.URL.RawQuery)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", service.ContentTypeOrJSON(contentType))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) ProxyMetadata(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")

	if strings.HasPrefix(id, "external-") {
		trackID := strings.TrimPrefix(id, "external-")

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		res, err := h.songService.GetSong(ctx, trackID)
		if err != nil {
			http.Error(w, "Song not found", http.StatusNotFound)
			return
		}

		resp := map[string]any{
			"subsonic-response": map[string]any{
				"status":  "ok",
				"version": "1.16.1",
				"song":    res,
			},
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(resp)
		return
	}

	h.rp.ServeHTTP(w, r)
}

func (h *Handler) NativeApiSong(w http.ResponseWriter, r *http.Request) {
	// Extract search query from Navidrome native REST parameters
	query := r.URL.Query().Get("title")
	if query == "" {
		query = r.URL.Query().Get("q")
	}
	if query == "" {
		query = r.URL.Query().Get("query")
	}
	if query == "" {
		query = r.URL.Query().Get("search")
	}
	if query == "" {
		query = r.URL.Query().Get("artist")
	}

	// When browsing local library without a search query, proxy directly to Navidrome
	if query == "" || query == "\"\"" {
		h.rp.ServeHTTP(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	// 1. Send request to Navidrome with incoming client headers to fetch local matching songs
	body, _, _, err := h.rp.SendNavidromeRequestWithHeaders(ctx, r.URL.Path, r.URL.RawQuery, r.Header)
	var localSongs []map[string]any
	if err == nil && body != nil {
		_ = json.Unmarshal(body, &localSongs)
	}

	// 2. Search external metadata provider for missing tracks
	externalSongs, err := h.metadata.SearchSongs(ctx, query)
	if err != nil {
		externalSongs = nil
	}

	allSongs := make([]map[string]any, 0, len(localSongs)+len(externalSongs))
	allSongs = append(allSongs, localSongs...)

	for _, s := range externalSongs {
		extMap := map[string]any{
			"id":          s.ID,
			"title":       s.Title,
			"artist":      s.Artist,
			"artistId":    s.ArtistID,
			"album":       s.Album,
			"albumId":     s.AlbumID,
			"duration":    s.Duration,
			"genre":       s.Genre,
			"coverArt":    s.CoverArt,
			"hasCoverArt": true,
			"size":        s.Size,
			"isDir":       false,
			"path":        s.Title + ".mp3",
		}
		allSongs = append(allSongs, extMap)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Total-Count", fmt.Sprintf("%d", len(allSongs)))
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(allSongs)
}

func (h *Handler) ProxyStream(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		id = strings.TrimPrefix(r.URL.Path, "/api/stream/")
		id = strings.TrimPrefix(id, "/api/raw/")
	}
	permanent := strings.Contains(r.URL.Path, "download")

	if strings.HasPrefix(id, "external-") {
		trackID := strings.TrimPrefix(id, "external-")
		_, targetPath, err := h.streamService.DownloadTrack(trackID, permanent)
		if err != nil {
			http.Error(w, "Failed to prepare track for streaming", http.StatusInternalServerError)
			return
		}

		// Trigger background scan asynchronously so Navidrome indexes the track without delaying playback
		go h.rp.SendNavidromeRequest(context.Background(), "/rest/startScan.view", r.URL.RawQuery)

		log.Printf("Directly streaming track for %s from file: %s", trackID, targetPath)
		http.ServeFile(w, r, targetPath)
		return
	}
	h.rp.ServeHTTP(w, r)
}

func (h *Handler) ProxyPlaylist(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("songIdToAdd")

	if strings.HasPrefix(id, "external-") {
		id = strings.TrimPrefix(id, "external-")

		songMetadata, _, err := h.streamService.DownloadTrack(id, true)
		if err != nil {
			http.Error(w, "Failed to prepare track for streaming", http.StatusInternalServerError)
			return
		}
		subsonicUser := r.URL.Query().Get("u")
		subsonicPass := r.URL.Query().Get("p")
		if subsonicUser == "" && subsonicPass == "" {
			http.Error(w, "Failed to get auth parameters", http.StatusInternalServerError)
			return
		}

		title := strings.TrimSuffix(songMetadata.Title, " (external)")
		artist := songMetadata.Artist
		mbid := songMetadata.MusicBrainzId
		if mbid == "" {
			mbid = id
			if strings.HasPrefix(mbid, "external-") {
				mbid = strings.TrimPrefix(mbid, "external-")
			}
		}

		foundSong, err := h.rp.FindNavidromeSongID(artist, title, mbid, r)
		if err != nil {
			http.Error(w, "Failed to find song in Navidrome", http.StatusInternalServerError)
			return
		}
		q := r.URL.Query()
		q.Set("songIdToAdd", foundSong.ID)
		r.URL.RawQuery = q.Encode()
		h.rp.ServeHTTP(w, r)
		return
	}
	h.rp.ServeHTTP(w, r)
}

func (h *Handler) ProxyCoverArt(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	id := r.URL.Query().Get("id")
	if id == "" {
		id = strings.TrimPrefix(r.URL.Path, "/api/coverArt/")
	}
	size := r.URL.Query().Get("size")

	if size == "" {
		size = "250"
	}
	if strings.HasPrefix(id, "external-") {
		trackId := strings.TrimPrefix(id, "external-")
		sizeInt, err := strconv.ParseInt(size, 10, 64)
		if err != nil {
			http.Error(w, "Invalid argument, size must be a number", http.StatusBadRequest)
			return
		}

		image, contentType, err := h.songService.GetCoverArt(ctx, trackId, sizeInt)
		if err != nil {
			http.Error(w, "Failed to fetch cover", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(image)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(image)
		return
	}

	h.rp.ServeHTTP(w, r)
}

func (h *Handler) GetAlbum(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	albumId := r.URL.Query().Get("id")

	resp, err := h.albumService.GetAlbum(ctx, albumId, r.URL.Path, r.URL.RawQuery)
	if err != nil {
		log.Printf("GetAlbum error: %v", err)
		http.Error(w, "Failed to fetch album", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func (h *Handler) CatchAll(w http.ResponseWriter, r *http.Request) {
	h.rp.ServeHTTP(w, r)
}
