package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/GerardPolloRebozado/navitube/src/client"
	"github.com/GerardPolloRebozado/navitube/src/config"
	"github.com/GerardPolloRebozado/navitube/src/model"
	"github.com/GerardPolloRebozado/navitube/src/service"
	"github.com/GerardPolloRebozado/navitube/src/util"
	"github.com/torabit/itunes"
)

type Handler struct {
	cfg *config.Config
	rp  *httputil.ReverseProxy
}

func NewHandler(cfg *config.Config, rp *httputil.ReverseProxy) *Handler {
	return &Handler{cfg: cfg, rp: rp}
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) SmartSearch(w http.ResponseWriter, r *http.Request) {
	if h.cfg.NavidromeBase == "" {
		http.Error(w, "server not configured (missing NAVIDROME_BASE)", http.StatusInternalServerError)
		return
	}

	// Build navidrome url
	var upstreamAbs strings.Builder
	upstreamAbs.WriteString(strings.TrimRight(h.cfg.NavidromeBase, "/"))
	upstreamAbs.WriteString(r.URL.Path)
	if raw := r.URL.RawQuery; raw != "" {
		upstreamAbs.WriteString("?")
		upstreamAbs.WriteString(raw)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	body, status, contentType, err := util.HTTPGet(ctx, upstreamAbs.String(), nil)
	if err != nil {
		log.Printf("upstream search error: %v", err)
		http.Error(w, "Upstream error", http.StatusBadGateway)
		return
	}

	var sr model.SubsonicSearchResponse
	hasSongs := false
	if strings.Contains(strings.ToLower(contentType), "json") {
		if err := json.Unmarshal(body, &sr); err == nil {
			if sr.Subsonic.SearchResult3 != nil {
				switch v := sr.Subsonic.SearchResult3.Song.(type) {
				case []any:
					if len(v) > 0 {
						hasSongs = true
					}
				case map[string]any:
					hasSongs = true
				case nil:
					hasSongs = false
				default:
					hasSongs = v != nil
				}
			}
		}
	}

	if hasSongs {
		w.Header().Set("Content-Type", service.ContentTypeOrJSON(contentType))
		w.WriteHeader(status)
		_, _ = w.Write(body)
		return
	}

	query := r.URL.Query().Get("query")
	if query == "" {
		query = r.URL.Query().Get("any")
	}

	items, err := service.PerformSmartSearch(r.Context(), query)
	if err != nil {
		log.Printf("SmartSearch error: %v", err)
	}

	resp := service.WrapExternalSearch(items)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *Handler) ProxyMetadata(w http.ResponseWriter, r *http.Request) {
	itunesClient := client.GetItunesClient()
	id := r.URL.Query().Get("id")

	if strings.HasPrefix(id, "itunes-") {
		trackID := strings.TrimPrefix(id, "itunes-")
		trackIDInt, err := strconv.ParseInt(trackID, 10, 64)
		if err != nil {
			http.Error(w, "Invalid song id", http.StatusBadRequest)
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		res, err := itunesClient.Lookup(ctx,
			itunes.ID(trackIDInt))

		if err != nil || len(res.Results) == 0 {
			http.Error(w, "Song not found", http.StatusNotFound)
			return
		}
		results := res.Results
		track := results[0]

		song := service.ItunesSongToSubsonicSong(track)

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

	h.rp.ServeHTTP(w, r)
}

func (h *Handler) handleITunesDownload(ctx context.Context, fullID string, authParams url.Values, permanent bool) {
	itunesClient := client.GetItunesClient()

	trackID := strings.TrimPrefix(fullID, "itunes-")
	trackIDInt, err := strconv.ParseInt(trackID, 10, 64)
	if err != nil {
		log.Printf("Background download failed lookup for %s: %v", trackID, err)
		return
	}

	bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := itunesClient.Lookup(bgCtx,
		itunes.ID(trackIDInt))

	if err != nil || len(res.Results) == 0 {
		log.Printf("Background download failed lookup for %s: %v", trackID, err)
		return
	}
	results := res.Results

	track := results[0]

	fullPath := util.GetTrackPath(h.cfg, track.ArtistName, track.CollectionName, track.TrackName, permanent)
	coverURL := track.ArtworkUrl100

	service.DownloadTrack(h.cfg, track.ArtistName, track.CollectionName, track.TrackName, fullPath, coverURL, authParams)
}

func (h *Handler) ProxyStream(w http.ResponseWriter, r *http.Request) {
	itunesClient := client.GetItunesClient()
	id := r.URL.Query().Get("id")
	permanent := strings.Contains(r.URL.Path, "download")

	if strings.HasPrefix(id, "itunes-") {
		trackID := strings.TrimPrefix(id, "itunes-")
		trackIDInt, err := strconv.ParseInt(trackID, 10, 64)
		if err != nil {
			http.Error(w, "Invalid song id", http.StatusBadRequest)
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		res, err := itunesClient.Lookup(ctx,
			itunes.ID(trackIDInt))

		if err != nil || len(res.Results) == 0 {
			log.Printf("iTunes lookup failed for ID %s: %v", trackID, err)
			http.Error(w, "Track not found", http.StatusNotFound)
			return
		}
		results := res.Results
		track := results[0]

		fullPath := util.GetTrackPath(h.cfg, track.ArtistName, track.CollectionName, track.TrackName, permanent)
		_ = os.MkdirAll(filepath.Dir(fullPath), 0755)

		if _, err := os.Stat(fullPath); err == nil {
			log.Printf("Serving existing file: %s", fullPath)
			http.ServeFile(w, r, fullPath)
			return
		}

		log.Printf("Downloading and streaming: %s - %s", track.ArtistName, track.TrackName)
		searchQuery := fmt.Sprintf("ytsearch1:%s - %s Audio", track.ArtistName, track.TrackName)

		cmd := exec.CommandContext(r.Context(), h.cfg.YTDLPPath,
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

		go h.handleITunesDownload(context.Background(), id, r.URL.Query(), permanent)
		return
	}

	h.rp.ServeHTTP(w, r)
}

func (h *Handler) ProxyPlaylistOrQueue(w http.ResponseWriter, r *http.Request) {
	ids := []string{}
	ids = append(ids, r.URL.Query()["songId"]...)
	ids = append(ids, r.URL.Query()["songIdToAdd"]...)
	ids = append(ids, r.URL.Query()["id"]...)

	hasItunes := false
	authParams := r.URL.Query()
	permanent := strings.Contains(r.URL.Path, "Playlist")

	for _, id := range ids {
		if strings.HasPrefix(id, "itunes-") {
			hasItunes = true
			go h.handleITunesDownload(context.Background(), id, authParams, permanent)
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
	h.rp.ServeHTTP(w, r)
}

func (h *Handler) ProxyCoverArt(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if strings.HasPrefix(id, "itunes-cover-") {
		rawURL := strings.TrimPrefix(id, "itunes-cover-")
		decodedURL, err := url.QueryUnescape(rawURL)
		if err != nil {
			http.Error(w, "Invalid cover URL", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		body, status, contentType, err := util.HTTPGet(ctx, decodedURL, nil)
		if err != nil {
			log.Printf("Error fetching cover art: %v", err)
			http.Error(w, "Failed to fetch cover", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		w.WriteHeader(status)
		_, _ = w.Write(body)
		return
	} else if strings.HasPrefix(id, "itunes-") {
		itunesClient := client.GetItunesClient()

		trackIDInt, err := strconv.ParseInt(strings.TrimPrefix(id, "itunes-"), 10, 64)
		if err != nil {
			http.Error(w, "Invalid song id", http.StatusBadRequest)
		}

		res, err := itunesClient.Lookup(r.Context(),
			itunes.ID(trackIDInt))

		if err != nil || len(res.Results) == 0 {
			log.Printf("Error fetching cover art: %v", err)
			http.Error(w, "Failed to fetch cover", http.StatusBadGateway)
			return
		}

		results := res.Results

		body, status, contentType, err := util.HTTPGet(r.Context(), results[0].ArtworkUrl100, nil)
		if err != nil {
			log.Printf("Error fetching cover art: %v", err)
			http.Error(w, "Failed to fetch cover", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
		w.WriteHeader(status)
		_, _ = w.Write(body)
		return
	}

	h.rp.ServeHTTP(w, r)
}

func (h *Handler) GetAlbum(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	albumId := r.URL.Query().Get("id")
	isAlbumExternal := strings.HasPrefix(albumId, "itunes-")

	var subsonicAlbumResponse model.SubsonicAlbumResponse
	var itunesSongs []itunes.Result
	var err error

	if isAlbumExternal {
		//get all the songs from itunes as album doesn't exist locally
		idInt, err := strconv.ParseInt(strings.TrimPrefix(albumId, "itunes-"), 10, 64)
		if err != nil {
			log.Printf("Cannot parse string id to integer %s: %v", albumId, err)
			http.Error(w, "Bad request: cannot parse id", http.StatusBadRequest)
			return
		}

		itunesSongs, err = service.PerformAlbumSongSearch(ctx, idInt)
		if err != nil || len(itunesSongs) == 0 {
			log.Printf("Error searching for this album %s: %v", albumId, err)
			http.Error(w, "Album not found", http.StatusNotFound)
			return
		}

		subsonicAlbumResponse.Subsonic.Status = "ok"
		subsonicAlbumResponse.Subsonic.Version = "1.16.1"
		album := service.ItunesAlbumToSubsonicAlbum(itunesSongs[0])
		subsonicAlbumResponse.Subsonic.Album = &album
		subsonicAlbumResponse.Subsonic.Album.Song = []model.SubsonicSong{}
	} else {
		// get an existing album from navidrome to append missing songs
		var upstreamAbs strings.Builder
		upstreamAbs.WriteString(strings.TrimRight(h.cfg.NavidromeBase, "/"))
		upstreamAbs.WriteString(r.URL.Path)
		if raw := r.URL.RawQuery; raw != "" {
			upstreamAbs.WriteString("?")
			upstreamAbs.WriteString(raw)
		}

		body, _, _, err := util.HTTPGet(ctx, upstreamAbs.String(), nil)
		if err != nil {
			log.Printf("upstream search error: %v", err)
			http.Error(w, "Upstream error", http.StatusBadGateway)
			return
		}

		if err := json.Unmarshal(body, &subsonicAlbumResponse); err != nil {
			log.Printf("error unmarshaling album response: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if subsonicAlbumResponse.Subsonic.Album == nil {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
			return
		}

		itunesAlbumList, err := service.PerformAlbumSearch(ctx, subsonicAlbumResponse.Subsonic.Album.Name)
		if err == nil && len(itunesAlbumList) > 0 {
			itunesSongs, _ = service.PerformAlbumSongSearch(ctx, itunesAlbumList[0].CollectionId)
		}
	}

	// Merge iTunes songs into the album response
	if subsonicAlbumResponse.Subsonic.Album != nil {
		for _, song := range itunesSongs {
			if song.WrapperType == "track" && !util.IsSongInSubsonicSongList(song.TrackName, subsonicAlbumResponse.Subsonic.Album.Song) {
				subsonicAlbumResponse.Subsonic.Album.Song = append(subsonicAlbumResponse.Subsonic.Album.Song, service.ItunesSongToSubsonicSong(song))
			}
		}
		subsonicAlbumResponse.Subsonic.Album.SongCount = int64(len(subsonicAlbumResponse.Subsonic.Album.Song))
	}

	respBody, err := json.Marshal(subsonicAlbumResponse)
	if err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(respBody)
}

func (h *Handler) CatchAll(w http.ResponseWriter, r *http.Request) {
	h.rp.ServeHTTP(w, r)
}
