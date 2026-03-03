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
	TriggerNavidromeScan()
}

var subsonicReverseProxyInstance *SubsonicReverseProxy

func GetSubsonicReverseProxy() *SubsonicReverseProxy {
	if subsonicReverseProxyInstance == nil {
		log.Fatal("Reverse proxy not initialized")
	}
	return subsonicReverseProxyInstance
}

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

func (p *SubsonicReverseProxy) TriggerNavidromeScan() {
	scanURL := fmt.Sprintf("%s/rest/startScan.view", strings.TrimRight(p.base, "/"))

	log.Println("Triggering Navidrome library scan...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, status, _, err := util.HTTPGet(ctx, scanURL, nil)
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
