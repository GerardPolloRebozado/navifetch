package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
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
	p, err := metadata.NewProvider(cfg.MetadataProvider, cfg.Country, cfg.Limit)
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
	h.TranslateExternalIDs(r)

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

	// When browsing local library or querying a specific song ID without a search query, proxy directly to Navidrome
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
	h.TranslateExternalIDs(r)
	h.rp.ServeHTTP(w, r)
}

func (h *Handler) ProxyPlaylist(w http.ResponseWriter, r *http.Request) {
	h.TranslateExternalIDs(r)
	h.rp.ServeHTTP(w, r)
}

func (h *Handler) ProxyIDTranslation(w http.ResponseWriter, r *http.Request) {
	h.TranslateExternalIDs(r)
	h.rp.ServeHTTP(w, r)
}

// TranslateExternalIDs transparently intercepts any query params, path params, or JSON body containing 'external-' IDs,
// ensures the track is downloaded, resolves its real Navidrome song ID, and rewrites the request before proxying.
func (h *Handler) TranslateExternalIDs(r *http.Request) {
	// 1. Process Query Parameters (e.g. id, mediaId, songId, songIdToAdd)
	q := r.URL.Query()
	modifiedQuery := false

	for key, values := range q {
		newValues := make([]string, len(values))
		for i, val := range values {
			if strings.HasPrefix(val, "external-") {
				cleanID := strings.TrimPrefix(val, "external-")
				songMetadata, targetPath, err := h.streamService.DownloadTrack(cleanID, true)
				if err == nil {
					title := strings.TrimSuffix(songMetadata.Title, " (external)")
					artist := songMetadata.Artist
					mbid := songMetadata.MusicBrainzId
					if mbid == "" {
						mbid = cleanID
					}
					relativePath := strings.TrimPrefix(targetPath, h.cfg.MusicLibraryPath+"/")
					foundSong, err := h.rp.FindNavidromeSongID(h.cfg, relativePath, artist, title, mbid, r)
					if err == nil {
						newValues[i] = foundSong.ID
						modifiedQuery = true
						continue
					}
				}
			}
			newValues[i] = val
		}
		q[key] = newValues
	}

	if modifiedQuery {
		r.URL.RawQuery = q.Encode()
	}

	// Process Path Parameters (e.g. /api/song/external-1839202495)
	if strings.Contains(r.URL.Path, "external-") {
		extIDRegex := regexp.MustCompile(`external-[a-zA-Z0-9_\-]+`)
		matches := extIDRegex.FindAllString(r.URL.Path, -1)
		for _, extID := range matches {
			cleanID := strings.TrimPrefix(extID, "external-")
			songMetadata, targetPath, err := h.streamService.DownloadTrack(cleanID, true)
			if err == nil {
				title := strings.TrimSuffix(songMetadata.Title, " (external)")
				artist := songMetadata.Artist
				mbid := songMetadata.MusicBrainzId
				if mbid == "" {
					mbid = cleanID
				}
				relativePath := strings.TrimPrefix(targetPath, h.cfg.MusicLibraryPath+"/")
				foundSong, err := h.rp.FindNavidromeSongID(h.cfg, relativePath, artist, title, mbid, r)
				if err == nil {
					r.URL.Path = strings.Replace(r.URL.Path, extID, foundSong.ID, 1)
				}
			}
		}
	}

	// Process Request Body (JSON or Form POST/PUT/PATCH)
	if r.Body != nil && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err == nil && len(bodyBytes) > 0 {
			bodyStr := string(bodyBytes)
			if strings.Contains(bodyStr, "external-") {
				extIDRegex := regexp.MustCompile(`external-[a-zA-Z0-9_\-]+`)
				matches := extIDRegex.FindAllString(bodyStr, -1)

				uniqueIDs := make(map[string]bool)
				for _, match := range matches {
					uniqueIDs[match] = true
				}

				replacements := make(map[string]string)
				for extID := range uniqueIDs {
					cleanID := strings.TrimPrefix(extID, "external-")
					songMetadata, targetPath, err := h.streamService.DownloadTrack(cleanID, true)
					if err == nil {
						title := strings.TrimSuffix(songMetadata.Title, " (external)")
						artist := songMetadata.Artist
						mbid := songMetadata.MusicBrainzId
						if mbid == "" {
							mbid = cleanID
						}
						relativePath := strings.TrimPrefix(targetPath, h.cfg.MusicLibraryPath+"/")
						foundSong, err := h.rp.FindNavidromeSongID(h.cfg, relativePath, artist, title, mbid, r)
						if err == nil {
							replacements[extID] = foundSong.ID
						}
					}
				}

				for extID, newID := range replacements {
					bodyStr = strings.ReplaceAll(bodyStr, extID, newID)
				}

				newBodyBytes := []byte(bodyStr)
				r.Body = io.NopCloser(bytes.NewReader(newBodyBytes))
				r.ContentLength = int64(len(newBodyBytes))
				r.Header.Set("Content-Length", strconv.Itoa(len(newBodyBytes)))
			} else {
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}
	}
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
