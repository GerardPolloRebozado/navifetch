package main

import (
	"log"
	"net/http"
	"time"

	"github.com/GerardPolloRebozado/navitube/src/api"
	"github.com/GerardPolloRebozado/navitube/src/client"
	"github.com/GerardPolloRebozado/navitube/src/config"
	"github.com/GerardPolloRebozado/navitube/src/service"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("config load error: %v", err)
	}

	rp, err := client.NewReverseProxy(cfg.NavidromeBase)
	if err != nil {
		log.Fatalf("proxy creation error: %v", err)
	}

	h := api.NewHandler(cfg, rp)
	mux := http.NewServeMux()

	api.RegisterRoutes(mux, h)

	// Middleware
	handler := api.LoggingMiddleware(mux)
	handler = api.CORSMiddleware(handler)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start background services
	service.StartCleanupCron(cfg)

	log.Printf("Proxy listening on :%s, forwarding to %s", cfg.Port, cfg.NavidromeBase)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
