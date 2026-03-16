package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/GerardPolloRebozado/navifetch/src/model"
	"github.com/GerardPolloRebozado/navifetch/src/util"
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
	var urlBuilder strings.Builder
	urlBuilder.WriteString(strings.TrimRight(p.base, "/"))
	urlBuilder.WriteString(path)
	if rawQuery != "" {
		urlBuilder.WriteString("?")
		urlBuilder.WriteString(rawQuery)
	}
	log.Printf("URL for Navidrome request: %s", urlBuilder.String())
	body, status, contentType, err := util.HTTPGet(ctx, urlBuilder.String(), nil)
	if err != nil {
		log.Printf("Navidrome request error: %v", err)
		return nil, 0, "", err
	}
	return body, status, contentType, nil
}

func (p *SubsonicReverseProxy) SearchNavidrome(ctx context.Context, path, rawQuery string) ([]model.SubsonicSong, string, error) {
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

func (p *SubsonicReverseProxy) FindNavidromeSongID(artist string, title string, mbid string, r *http.Request) (*model.SubsonicSong, error) {
	var foundSong *model.SubsonicSong
	query := fmt.Sprintf("%s %s", artist, title)

	// Use background context with timeout for Navidrome searches to avoid cancellation if a client disconnects
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	searchParams := r.URL.Query()
	searchParams.Set("query", query)
	searchRawQuery := searchParams.Encode()

	log.Printf("Searching Navidrome for exact match: %s (MBID: %s)", query, mbid)

	for i := 0; i < 15; i++ {
		searchResult, _, err := p.SearchNavidrome(ctx, "/rest/search3.view", searchRawQuery)
		if err == nil {
			for _, song := range searchResult {
				// 1. Try match by MBID if available
				if mbid != "" && song.MusicBrainzId == mbid {
					log.Printf("Found exact match in Navidrome by MBID: %s (ID: %s)", song.Title, song.ID)
					foundSong = &song
					break
				}
				// 2. Try match by Artist and Title as fallback
				if strings.EqualFold(song.Artist, artist) && strings.EqualFold(song.Title, title) {
					log.Printf("Found match in Navidrome by Artist/Title: %s - %s (ID: %s)", song.Artist, song.Title, song.ID)
					foundSong = &song
					break
				}
			}
		}

		if foundSong != nil {
			return foundSong, nil
		}

		if i < 14 {
			log.Printf("Match not found yet, retrying in 2s... (attempt %d/15)", i+1)
			time.Sleep(2 * time.Second)
			// Re-trigger scan
			go p.SendNavidromeRequest(context.Background(), "/rest/startScan.view", r.URL.RawQuery)
		}
	}

	log.Printf("Failed to find exact match, falling back to first search result for: %s", title)
	searchResult, _, err := p.SearchNavidrome(ctx, "/rest/search3.view", searchRawQuery)
	if err == nil && len(searchResult) > 0 {
		return &searchResult[0], nil
	}

	return nil, fmt.Errorf("song not found in Navidrome after download")
}
