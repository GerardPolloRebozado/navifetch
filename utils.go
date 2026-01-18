package main

import (
	"context"
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

func GetTrackPath(cfg *Config, artist, album, title string, permanent bool) string {
	safeArtist := sanitizeFilename(artist)
	safeAlbum := sanitizeFilename(album)
	safeTitle := sanitizeFilename(title)
	folder := "cached"
	if permanent {
		folder = "downloads"
	}

	dir := filepath.Join(cfg.MusicLibraryPath, folder, safeArtist, safeAlbum)
	filename := fmt.Sprintf("%s.mp3", safeTitle)
	return filepath.Join(dir, filename)
}

func download(ytdlpPath, navidromeBase, artist, album, title, targetPath, coverURL string, authParams url.Values) {
	if _, err := os.Stat(targetPath); err == nil {
		return
	}

	log.Printf("Saving permanent copy for Navidrome: %s", targetPath)
	_ = os.MkdirAll(filepath.Dir(targetPath), 0755)

	coverPath := ""
	if coverURL != "" {
		tmpCover, err := os.CreateTemp("", "cover-*.jpg")
		if err == nil {
			defer os.Remove(tmpCover.Name())
			defer tmpCover.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			body, status, _, err := HTTPGet(ctx, coverURL, nil)
			if err == nil && status == 200 {
				_, _ = tmpCover.Write(body)
				coverPath = tmpCover.Name()
			}
		}
	}

	searchQuery := fmt.Sprintf("ytsearch1:%s - %s Audio", artist, title)

	safeTitle := strings.ReplaceAll(title, "'", "")
	safeArtist := strings.ReplaceAll(artist, "'", "")
	safeAlbum := strings.ReplaceAll(album, "'", "")

	ffmpegArgs := fmt.Sprintf("ffmpeg:-metadata title='%s' -metadata artist='%s' -metadata album='%s'", safeTitle, safeArtist, safeAlbum)

	args := []string{
		"-x", "--audio-format", "mp3",
		"--add-metadata",
		"--postprocessor-args", ffmpegArgs,
		"-o", targetPath,
		"--no-playlist",
	}
	if coverPath != "" {
		args = append(args, "--embed-thumbnail")
	} else {
		args = append(args, "--embed-thumbnail")
	}

	args = append(args, searchQuery)

	cmd := exec.Command(ytdlpPath, args...)

	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Failed to save permanent copy: %v\nOutput: %s", err, string(output))
	} else {
		log.Printf("Successfully saved: %s", targetPath)
		TriggerNavidromeScan(navidromeBase, authParams)
	}
}

func TriggerNavidromeScan(base string, params url.Values) {
	scanURL := fmt.Sprintf("%s/rest/startScan.view?%s", strings.TrimRight(base, "/"), params.Encode())

	log.Println("Triggering Navidrome library scan...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, status, _, err := HTTPGet(ctx, scanURL, nil)
	if err != nil {
		log.Printf("Failed to trigger scan: %v", err)
		return
	}
	if status != 200 {
		log.Printf("Navidrome scan trigger returned status: %d", status)
	} else {
		log.Println("Navidrome scan triggered successfully.")
	}
}

func sanitizeFilename(s string) string {
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
