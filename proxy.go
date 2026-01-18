package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
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
		req.URL.Path = joinPath(target.Path, req.URL.Path)
		req.Header.Set("X-Forwarded-Host", req.Host)
		if req.TLS != nil {
			req.Header.Set("X-Forwarded-Proto", "https")
		} else {
			req.Header.Set("X-Forwarded-Proto", "http")
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("proxy error for %s: %v", r.URL.String(), err)
		http.Error(w, "Upstream error", http.StatusBadGateway)
	}

	return proxy, nil
}

func joinPath(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
