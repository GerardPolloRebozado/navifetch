package api

import (
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("/healthz", h.Healthz)

	mux.HandleFunc("/rest/search3.view", h.SmartSearch)
	mux.HandleFunc("/rest/search2.view", h.SmartSearch)
	mux.HandleFunc("/rest/search3", h.SmartSearch)
	mux.HandleFunc("/rest/search2", h.SmartSearch)
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

	// Catch-all reverse proxy
	mux.HandleFunc("/", h.CatchAll)
}
