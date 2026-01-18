package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

func SmartSearch(upstreamBase string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if upstreamBase == "" {
			http.Error(w, "server not configured (missing NAVIDROME_BASE)", http.StatusInternalServerError)
			return
		}

		// Build upstream absolute URL
		var upstreamAbs strings.Builder
		upstreamAbs.WriteString(strings.TrimRight(upstreamBase, "/"))
		upstreamAbs.WriteString(r.URL.Path)
		if raw := r.URL.RawQuery; raw != "" {
			upstreamAbs.WriteString("?")
			upstreamAbs.WriteString(raw)
		}

		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()

		body, status, contentType, err := HTTPGet(ctx, upstreamAbs.String(), nil)
		if err != nil {
			log.Printf("upstream search error: %v", err)
			http.Error(w, "Upstream error", http.StatusBadGateway)
			return
		}

		type searchResult3 struct {
			Song any `json:"song"`
		}
		type subResp struct {
			Subsonic struct {
				Status        string         `json:"status"`
				Version       string         `json:"version"`
				SearchResult3 *searchResult3 `json:"searchResult3,omitempty"`
			} `json:"subsonic-response"`
		}

		var sr subResp
		hasSongs := false
		if strings.Contains(strings.ToLower(contentType), "json") {
			if err := json.Unmarshal(body, &sr); err == nil {
				if sr.Subsonic.SearchResult3 != nil {
					switch v := sr.Subsonic.SearchResult3.Song.(type) {
					case []any:
						if len(v) > 0 {
							hasSongs = true
						}
					case map[string]any:
						hasSongs = true
					case nil:
						hasSongs = false
					default:
						hasSongs = v != nil
					}
				}
			}
		}

		if hasSongs {
			w.Header().Set("Content-Type", contentTypeOrJSON(contentType))
			w.WriteHeader(status)
			_, _ = w.Write(body)
			return
		}

		query := r.URL.Query().Get("query")
		typ := r.URL.Query().Get("type")
		if query == "" {
			query = r.URL.Query().Get("any")
		}
		if typ == "" {
			typ = "song"
		}

		artist, title := splitArtistTitle(query)

		mbCtx, mbCancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer mbCancel()

		mbQuery := query
		if artist != "" && title != "" {
			mbQuery = `artist:"` + artist + `" AND recording:"` + title + `"`
		}

		recs, err := ITunesSearch(mbCtx, mbQuery)
		if err != nil {
			log.Printf("itunes search error: %v", err)
		}

		var externalItems []ExternalItem
		for _, rec := range recs {
			item := ExternalItem{
				Source:        "itunes",
				RecordingID:   fmt.Sprintf("%d", rec.TrackId),
				Title:         rec.TrackName,
				Artist:        rec.ArtistName,
				Album:         rec.CollectionName,
				ReleaseID:     fmt.Sprintf("%d", rec.CollectionId),
				CoverArtURL:   GetHighResArtwork(rec.ArtworkUrl100),
				Confidence:    1.0,
				OriginalQuery: query,
				Duration:      rec.TrackTimeMillis / 1000,
			}
			externalItems = append(externalItems, item)
		}

		resp := WrapExternalSearch(externalItems)

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func splitArtistTitle(q string) (artist, title string) {
	parts := strings.Split(q, " - ")
	if len(parts) >= 2 {
		artist = strings.TrimSpace(parts[0])
		title = strings.TrimSpace(strings.Join(parts[1:], " - "))
		return artist, title
	}
	if i := strings.Index(q, `" "`); i != -1 {
		artist = strings.TrimSpace(q[:i])
		title = strings.Trim(strings.TrimSpace(q[i+1:]), `"`)
		return artist, title
	}
	return "", ""
}

func contentTypeOrJSON(ct string) string {
	if ct != "" {
		return ct
	}
	return "application/json; charset=utf-8"
}
