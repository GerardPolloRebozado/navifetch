package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/GerardPolloRebozado/navifetch/src/config"
	"github.com/GerardPolloRebozado/navifetch/src/model"
	"github.com/GerardPolloRebozado/navifetch/src/util"
	_ "modernc.org/sqlite"
)

type SubsonicReverseProxy struct {
	base  string
	proxy *httputil.ReverseProxy
}

type NavidromeClient interface {
	SendNavidromeRequest(ctx context.Context, path, rawQuery string) ([]byte, int, string, error)
	SearchNavidrome(ctx context.Context, path, rawQuery string) ([]model.SubsonicSong, string, error)
}

var subsonicReverseProxyInstance *SubsonicReverseProxy

func NewSubsonicReverseProxy(base string) (*SubsonicReverseProxy, error) {
	if subsonicReverseProxyInstance != nil {
		return subsonicReverseProxyInstance, nil
	}
	target, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)

	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		req.Header.Set("X-Forwarded-Host", req.Host)
		if req.TLS != nil {
			req.Header.Set("X-Forwarded-Proto", "https")
		} else {
			req.Header.Set("X-Forwarded-Proto", "http")
		}
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("Access-Control-Allow-Origin")
		resp.Header.Del("Access-Control-Allow-Methods")
		resp.Header.Del("Access-Control-Allow-Headers")
		resp.Header.Del("Access-Control-Expose-Headers")
		return nil
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("proxy error for %s: %v", r.URL.String(), err)
		http.Error(w, "Upstream error", http.StatusBadGateway)
	}

	subsonicReverseProxyInstance = &SubsonicReverseProxy{
		base:  base,
		proxy: proxy,
	}
	return subsonicReverseProxyInstance, nil
}

func (p *SubsonicReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.proxy.ServeHTTP(w, r)
}

func (p *SubsonicReverseProxy) SendNavidromeRequest(ctx context.Context, path, rawQuery string) ([]byte, int, string, error) {
	return p.SendNavidromeRequestWithHeaders(ctx, path, rawQuery, nil)
}

func (p *SubsonicReverseProxy) SendNavidromeRequestWithHeaders(ctx context.Context, path, rawQuery string, reqHeaders http.Header) ([]byte, int, string, error) {
	var urlBuilder strings.Builder
	urlBuilder.WriteString(strings.TrimRight(p.base, "/"))
	urlBuilder.WriteString(path)
	if rawQuery != "" {
		urlBuilder.WriteString("?")
		urlBuilder.WriteString(rawQuery)
	}
	headers := make(map[string]string)
	if reqHeaders != nil {
		for k, v := range reqHeaders {
			if len(v) > 0 {
				headers[k] = v[0]
			}
		}
	}
	log.Printf("URL for Navidrome request: %s", urlBuilder.String())
	body, status, contentType, err := util.HTTPGet(ctx, urlBuilder.String(), headers)
	if err != nil {
		log.Printf("Navidrome request error: %v", err)
		return nil, 0, "", err
	}
	return body, status, contentType, nil
}

func (p *SubsonicReverseProxy) SearchNavidrome(ctx context.Context, path, rawQuery string) ([]model.SubsonicSong, string, error) {
	if u, err := url.ParseQuery(rawQuery); err == nil {
		u.Set("f", "json")
		rawQuery = u.Encode()
	}
	body, _, contentType, err := p.SendNavidromeRequest(ctx, path, rawQuery)
	if err == nil && body != nil {
		var sr model.SubsonicSearchResponse
		if strings.Contains(strings.ToLower(contentType), "json") {
			if err := json.Unmarshal(body, &sr); err == nil {
				if sr.Subsonic.SearchResult3 != nil && len(sr.Subsonic.SearchResult3.Song) > 0 {
					return sr.Subsonic.SearchResult3.Song, contentType, nil
				}
				if len(sr.Subsonic.Song) > 0 {
					return sr.Subsonic.Song, contentType, nil
				}
			}
		}
		return nil, contentType, nil
	}
	return nil, "", err
}

// WaitForScanComplete polls getScanStatus until the scan finishes or the context expires
func (p *SubsonicReverseProxy) WaitForScanComplete(ctx context.Context, rawQuery string) error {
	for {
		body, _, _, err := p.SendNavidromeRequest(ctx, "/rest/getScanStatus.view", rawQuery+"&f=json")
		if err == nil {
			var status struct {
				Subsonic struct {
					ScanStatus struct {
						Scanning bool `json:"scanning"`
						Count    int  `json:"count"`
					} `json:"scanStatus"`
				} `json:"subsonic-response"`
			}
			if json.Unmarshal(body, &status) == nil && !status.Subsonic.ScanStatus.Scanning {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// findSongByPathInDB queries Navidrome's SQLite database directly to find a song by its file path
func findSongByPathInDB(dbPath, relativePath string) (*model.SubsonicSong, error) {
	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("failed to open Navidrome DB: %w", err)
	}
	defer db.Close()

	var song model.SubsonicSong
	err = db.QueryRow(
		"SELECT id, title, artist, album, COALESCE(mbz_recording_id, '') FROM media_file WHERE path = ?",
		relativePath,
	).Scan(&song.ID, &song.Title, &song.Artist, &song.Album, &song.MusicBrainzId)
	if err != nil {
		return nil, fmt.Errorf("song not found at path %s: %w", relativePath, err)
	}
	return &song, nil
}

// FindNavidromeSongID resolves a downloaded song's Navidrome ID:
// If Subsonic auth is present, trigger rescan via Subsonic API
// Poll direct SQLite DB lookup by file path
// Fallback: name-based search via search3 API
func (p *SubsonicReverseProxy) FindNavidromeSongID(cfg *config.Config, filePath string, artist string, title string, mbid string, r *http.Request) (*model.SubsonicSong, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	hasSubsonicAuth := r.URL.Query().Get("u") != ""

	if hasSubsonicAuth {
		log.Printf("Triggering Navidrome scan for new file: %s", filePath)
		go p.SendNavidromeRequest(context.Background(), "/rest/startScan.view", r.URL.RawQuery)
	}

	// DB lookup by path
	if cfg.NavidromeDBPath != "" && filePath != "" {
		log.Printf("Polling Navidrome DB for path: %s", filePath)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for i := 0; i < 30; i++ {
			song, err := findSongByPathInDB(cfg.NavidromeDBPath, filePath)
			if err == nil {
				log.Printf("Found song by DB path lookup: %s - %s (ID: %s)", song.Artist, song.Title, song.ID)
				return song, nil
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-ticker.C:
			}
		}
		log.Printf("DB path lookup timed out after 15s for: %s", filePath)
	}

	if !hasSubsonicAuth {
		return nil, fmt.Errorf("song not found in Navidrome DB after download: %s - %s", artist, title)
	}

	query := fmt.Sprintf("%s %s", artist, title)
	searchParams := r.URL.Query()
	searchParams.Set("query", query)
	searchRawQuery := searchParams.Encode()

	log.Printf("Searching Navidrome via search3 fallback for: %s (MBID: %s)", query, mbid)

	for i := 0; i < 5; i++ {
		searchResult, _, err := p.SearchNavidrome(ctx, "/rest/search3.view", searchRawQuery)
		if err == nil {
			for _, song := range searchResult {
				if mbid != "" && song.MusicBrainzId == mbid {
					log.Printf("Found match by MBID: %s (ID: %s)", song.Title, song.ID)
					return &song, nil
				}
				if strings.EqualFold(song.Artist, artist) && strings.EqualFold(song.Title, title) {
					log.Printf("Found match by Artist/Title: %s - %s (ID: %s)", song.Artist, song.Title, song.ID)
					return &song, nil
				}
			}
		}

		if i < 4 {
			log.Printf("Match not found yet, retrying in 2s... (attempt %d/5)", i+1)
			time.Sleep(2 * time.Second)
			go p.SendNavidromeRequest(context.Background(), "/rest/startScan.view", r.URL.RawQuery)
		}
	}

	return nil, fmt.Errorf("song not found in Navidrome after download: %s - %s", artist, title)
}
