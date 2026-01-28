package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// iTunes Search API response structs
type iTunesSearchResponse struct {
	ResultCount int            `json:"resultCount"`
	Results     []iTunesResult `json:"results"`
}

type iTunesResult struct {
	WrapperType      string `json:"wrapperType"`
	Kind             string `json:"kind"`
	ArtistId         int64  `json:"artistId"`
	CollectionId     int64  `json:"collectionId"`
	TrackId          int64  `json:"trackId"`
	ArtistName       string `json:"artistName"`
	CollectionName   string `json:"collectionName"`
	TrackName        string `json:"trackName"`
	ArtworkUrl100    string `json:"artworkUrl100"`
	ArtworkUrl60     string `json:"artworkUrl60"`
	PreviewUrl       string `json:"previewUrl"`
	ReleaseDate      string `json:"releaseDate"`
	PrimaryGenreName string `json:"primaryGenreName"`
	TrackTimeMillis  int64  `json:"trackTimeMillis"`
}

// ITunesSearch searches for songs to display metadata.
func ITunesSearch(ctx context.Context, query string) ([]iTunesResult, error) {

	endpoint := fmt.Sprintf(
		"https://itunes.apple.com/search?term=%s&media=music&entity=song",
		url.QueryEscape(query),
	)

	fmt.Printf("DEBUG: iTunes URL: %s\n", endpoint)

	body, status, _, err := HTTPGet(ctx, endpoint, nil)
	if err != nil {
		return nil, err
	}

	if status < 200 || status >= 300 {
		fmt.Printf("DEBUG: iTunes returned status %d. Body: %s\n", status, string(body))
		return nil, fmt.Errorf("itunes status %d", status)
	}

	var resp iTunesSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	fmt.Printf("DEBUG: Found %d results for query '%s'\n", resp.ResultCount, query)

	return resp.Results, nil
}

// ITunesLookup fetches details for a specific track by its iTunes ID.
func ITunesLookup(ctx context.Context, id string) ([]iTunesResult, error) {
	endpoint := fmt.Sprintf(
		"https://itunes.apple.com/lookup?id=%s&entity=song",
		url.QueryEscape(id),
	)

	fmt.Printf("DEBUG: iTunes Lookup URL: %s\n", endpoint)

	body, status, _, err := HTTPGet(ctx, endpoint, nil)
	if err != nil {
		return nil, err
	}

	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("itunes status %d", status)
	}

	var resp iTunesSearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return resp.Results, nil
}

// GetHighResArtwork tries to convert the 100x100 URL to a higher resolution.
func GetHighResArtwork(url string) string {
	if url == "" {
		return ""
	}
	return strings.Replace(url, "100x100bb", "600x600bb", 1)
}

// ExternalItem represents an ephemeral search result.
type ExternalItem struct {
	Source        string  `json:"source"`
	RecordingID   string  `json:"recordingId,omitempty"`
	Title         string  `json:"title,omitempty"`
	ArtistID      int64   `json:"artistId,omitempty"`
	Artist        string  `json:"artist,omitempty"`
	Album         string  `json:"album,omitempty"`
	AlbumID       int64   `json:"albumId,omitempty"`
	Genre         string  `json:"genre,omitempty"`
	ReleaseID     string  `json:"releaseId,omitempty"`
	CoverArtURL   string  `json:"coverArtUrl,omitempty"`
	Confidence    float64 `json:"confidence,omitempty"`
	OriginalQuery string  `json:"originalQuery,omitempty"`
	Duration      int64   `json:"duration,omitempty"` // Seconds
	Size          int64   `json:"size,omitempty"`
}

// WrapExternalSearch returns a Subsonic-compatible with the results in the "song" list.
func WrapExternalSearch(items []ExternalItem) map[string]any {
	songs := make([]map[string]any, len(items))
	for i, item := range items {
		songs[i] = map[string]any{
			"parent":                item.Album,
			"id":                    "itunes-" + item.RecordingID,
			"title":                 item.Title,
			"artist":                item.Artist,
			"artistId":              fmt.Sprintf("%d", item.ArtistID),
			"album":                 item.Album,
			"albumId":               fmt.Sprintf("%d", item.AlbumID),
			"genre":                 item.Genre,
			"coverArt":              "itunes-cover-" + url.QueryEscape(item.CoverArtURL),
			"duration":              item.Duration,
			"size":                  (item.Duration * 160000) / 8,
			"isDir":                 false,
			"isVideo":               false,
			"suffix":                "mp3",
			"contentType":           "audio/mpeg",
			"transcodedSuffix":      "mp3",
			"transcodedContentType": "audio/mpeg",
			"type":                  "music",
			"mediaType":             "song",
			"created":               time.Now(),
			"channelCount":          2,
			"bitDepth":              16,
			"samplingRate":          44100,
			"bpm":                   1,
			"comment":               item.Source,
			"sortName":              item.Title,
			"musicBrainzId":         "",
			"isrc":                  []string{},
			"genres": []string{
				item.Genre,
			},
			"artists": []string{
				item.Artist,
			},
			"displayArtist": item.Artist,
			"albumArtists": []string{
				item.Artist,
			},
			"displayAlbumArtists": item.Artist,
			"contributors":        []string{},
			"displayComposer":     item.Artist,
			"moods":               []string{},
			"replayGain":          0,
			"explicitStatus":      "clean",
		}
	}

	return map[string]any{
		"subsonic-response": map[string]any{
			"status":  "ok",
			"version": "1.16.1",
			"searchResult3": map[string]any{
				"song": songs,
			},
		},
	}
}

// ProxyCoverArt proxies cover art requests, fetching from iTunes if needed.
func ProxyCoverArt(rp http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if strings.HasPrefix(id, "itunes-cover-") {
			rawURL := strings.TrimPrefix(id, "itunes-cover-")
			decodedURL, err := url.QueryUnescape(rawURL)
			if err != nil {
				http.Error(w, "Invalid cover URL", http.StatusBadRequest)
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			// Fetch the image
			body, status, contentType, err := HTTPGet(ctx, decodedURL, nil)
			if err != nil {
				log.Printf("Error fetching cover art: %v", err)
				http.Error(w, "Failed to fetch cover", http.StatusBadGateway)
				return
			}

			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
			w.WriteHeader(status)
			_, _ = w.Write(body)
			return
		}

		// Forward to Navidrome
		rp.ServeHTTP(w, r)
	}
}
