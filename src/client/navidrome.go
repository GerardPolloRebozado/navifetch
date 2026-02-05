package client

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/GerardPolloRebozado/navitube/src/util"
)

func NewReverseProxy(base string) (*httputil.ReverseProxy, error) {
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

	return proxy, nil
}

func TriggerNavidromeScan(base string, params url.Values) {
	scanURL := fmt.Sprintf("%s/rest/startScan.view?%s", strings.TrimRight(base, "/"), params.Encode())

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
