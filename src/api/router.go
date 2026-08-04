package api

import (
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("/healthz", h.Healthz)

	mux.HandleFunc("/rest/search3.view", h.SmartSearch)
	mux.HandleFunc("/rest/search2.view", h.SmartSearch)
	mux.HandleFunc("/rest/search.view", h.SmartSearch)
	mux.HandleFunc("/rest/search3", h.SmartSearch)
	mux.HandleFunc("/rest/search2", h.SmartSearch)
	mux.HandleFunc("/rest/search", h.SmartSearch)
	mux.HandleFunc("/rest/getAlbum", h.GetAlbum)

	mux.HandleFunc("/rest/getCoverArt.view", h.ProxyCoverArt)
	mux.HandleFunc("/rest/getCoverArt", h.ProxyCoverArt)

	mux.HandleFunc("/rest/stream.view", h.ProxyStream)
	mux.HandleFunc("/rest/stream", h.ProxyStream)
	mux.HandleFunc("/rest/download.view", h.ProxyStream)
	mux.HandleFunc("/rest/download", h.ProxyStream)

	mux.HandleFunc("/rest/getSong.view", h.ProxyMetadata)
	mux.HandleFunc("/rest/getSong", h.ProxyMetadata)

	mux.HandleFunc("/rest/createPlaylist.view", h.ProxyPlaylist)
	mux.HandleFunc("/rest/createPlaylist", h.ProxyPlaylist)
	mux.HandleFunc("/rest/updatePlaylist.view", h.ProxyPlaylist)
	mux.HandleFunc("/rest/updatePlaylist", h.ProxyPlaylist)
	mux.HandleFunc("/rest/savePlayQueue.view", h.ProxyPlaylist)
	mux.HandleFunc("/rest/savePlayQueue", h.ProxyPlaylist)

	// Navidrome native API routes (used by Feishin and modern clients)
	mux.HandleFunc("/api/song", h.NativeApiSong)
	mux.HandleFunc("/api/song/", h.NativeApiSong)

	mux.HandleFunc("/api/coverArt", h.ProxyCoverArt)
	mux.HandleFunc("/api/coverArt/", h.ProxyCoverArt)

	mux.HandleFunc("/api/stream", h.ProxyStream)
	mux.HandleFunc("/api/stream/", h.ProxyStream)
	mux.HandleFunc("/api/raw", h.ProxyStream)
	mux.HandleFunc("/api/raw/", h.ProxyStream)

	// Catch-all reverse proxy
	mux.HandleFunc("/", h.CatchAll)
}
