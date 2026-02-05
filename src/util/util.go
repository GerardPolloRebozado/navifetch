package util

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/GerardPolloRebozado/navitube/src/config"
	"github.com/GerardPolloRebozado/navitube/src/model"
)

// HTTPGet is a simple helper to make GET requests.
func HTTPGet(ctx context.Context, url string, headers map[string]string) (body []byte, status int, contentType string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, resp.Header.Get("Content-Type"), err
	}
	return b, resp.StatusCode, resp.Header.Get("Content-Type"), nil
}

func GetTrackPath(cfg *config.Config, artist, album, title string, permanent bool) string {
	safeArtist := SanitizeFilename(artist)
	safeAlbum := SanitizeFilename(album)
	safeTitle := SanitizeFilename(title)
	folder := "cached"
	if permanent {
		folder = "downloads"
	}

	dir := filepath.Join(cfg.MusicLibraryPath, folder, safeArtist, safeAlbum)
	filename := fmt.Sprintf("%s.mp3", safeTitle)
	return filepath.Join(dir, filename)
}

func SanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, "\\", "-")
	s = strings.ReplaceAll(s, ":", "-")
	s = strings.ReplaceAll(s, "*", "-")
	s = strings.ReplaceAll(s, "?", "-")
	s = strings.ReplaceAll(s, "\"", "-")
	s = strings.ReplaceAll(s, "<", "-")
	s = strings.ReplaceAll(s, ">", "-")
	s = strings.ReplaceAll(s, "|", "-")
	return strings.TrimSpace(s)
}

func IsSongInSubsonicSongList(name string, subsonicList []model.SubsonicSong) bool {
	for _, song := range subsonicList {
		if strings.ToLower(name) == strings.ToLower(song.Title) {
			return true
		}
	}
	return false
}
