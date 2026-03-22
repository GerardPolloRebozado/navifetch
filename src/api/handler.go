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
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	log.Printf("Search query: %s", query)
	if query == "\"\"" || query == "" {
		h.rp.ServeHTTP(w, r)
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

func (h *Handler) ProxyStream(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	permanent := strings.Contains(r.URL.Path, "download")

	if strings.HasPrefix(id, "external-") {
		trackID := strings.TrimPrefix(id, "external-")
		songMetadata, _, err := h.streamService.DownloadTrack(trackID, permanent)
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
			mbid = trackID
			if strings.HasPrefix(mbid, "external-") {
				mbid = strings.TrimPrefix(mbid, "external-")
			}
		}

		foundSong, err := h.rp.FindNavidromeSongID(artist, title, mbid, r)
		if err != nil {
			http.Error(w, "Failed to find song in Navidrome", http.StatusInternalServerError)
			return
		}

		// Update request with the internal Navidrome ID
		q := r.URL.Query()
		q.Set("id", foundSong.ID)
		r.URL.RawQuery = q.Encode()
		h.rp.ServeHTTP(w, r)
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
