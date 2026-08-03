package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/GerardPolloRebozado/navifetch/src/model"
	"github.com/GerardPolloRebozado/navifetch/src/util"
)

type AggregatorProvider struct {
	itunes   Provider
	mb       Provider
	limit    int
	cache    sync.Map // string -> model.SubsonicSong
	coverMap sync.Map // string -> string (cover URL)
}

func NewAggregatorProvider(country string, limit int) *AggregatorProvider {
	return &AggregatorProvider{
		itunes: NewItunesProvider(country, limit),
		mb:     NewMusicBrainzProvider(limit),
		limit:  limit,
	}
}

func (a *AggregatorProvider) SearchSongs(ctx context.Context, query string) ([]model.SubsonicSong, error) {
	var songs []model.SubsonicSong
	seenIDs := make(map[string]bool)

	itunesSongs, err := a.itunes.SearchSongs(ctx, query)
	if err == nil {
		for _, s := range itunesSongs {
			if !seenIDs[s.ID] {
				seenIDs[s.ID] = true
				songs = append(songs, s)
				a.cache.Store(s.ID, s)
				if strings.HasPrefix(s.ID, "external-") {
					a.cache.Store(strings.TrimPrefix(s.ID, "external-"), s)
				}
				if s.CoverArt != "" {
					a.coverMap.Store(s.ID, s.CoverArt)
				}
			}
		}
	}

	ytSongs := a.searchYouTube(ctx, query)
	for _, s := range ytSongs {
		if !seenIDs[s.ID] {
			seenIDs[s.ID] = true
			songs = append(songs, s)
			a.cache.Store(s.ID, s)
			if strings.HasPrefix(s.ID, "external-") {
				a.cache.Store(strings.TrimPrefix(s.ID, "external-"), s)
			}
			if s.CoverArt != "" {
				a.coverMap.Store(s.ID, s.CoverArt)
			}
		}
	}

	return songs, nil
}

type YTDLPFlatItem struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Channel    string `json:"channel"`
	Uploader   string `json:"uploader"`
	Duration   int64  `json:"duration"`
	Thumbnails []struct {
		URL string `json:"url"`
	} `json:"thumbnails"`
}

func (a *AggregatorProvider) searchYouTube(ctx context.Context, query string) []model.SubsonicSong {
	cmdCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	searchArg := fmt.Sprintf("ytsearch%d:%s", 5, query)
	cmd := exec.CommandContext(cmdCtx, "yt-dlp", "--flat-playlist", "--dump-json", searchArg)

	output, err := cmd.Output()
	if err != nil {
		log.Printf("yt-dlp search error for query '%s': %v", query, err)
		return nil
	}

	lines := strings.Split(string(output), "\n")
	var songs []model.SubsonicSong

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var item YTDLPFlatItem
		if err := json.Unmarshal([]byte(line), &item); err != nil || item.ID == "" {
			continue
		}

		artist := item.Channel
		if artist == "" {
			artist = item.Uploader
		}
		if artist == "" {
			artist = "Unknown Artist"
		}

		coverURL := ""
		if len(item.Thumbnails) > 0 {
			coverURL = item.Thumbnails[len(item.Thumbnails)-1].URL
		}

		cleanID := "yt-" + item.ID
		externalID := "external-" + cleanID

		song := model.SubsonicSong{
			ID:                    externalID,
			Title:                 item.Title + " (external)",
			Artist:                artist,
			DisplayArtist:         artist,
			Album:                 "YouTube",
			AlbumID:               "external-yt-album",
			CoverArt:              externalID,
			Duration:              item.Duration,
			IsDir:                 false,
			ContentType:           "audio/mpeg",
			Suffix:                "mp3",
			TranscodedSuffix:      "mp3",
			TranscodedContentType: "audio/mpeg",
			Type:                  "music",
			MediaType:             "song",
			Created:               time.Now(),
			MusicBrainzId:         cleanID,
		}

		songs = append(songs, song)
		if coverURL != "" {
			a.coverMap.Store(externalID, coverURL)
			a.coverMap.Store(cleanID, coverURL)
		}
	}

	return songs
}

func (a *AggregatorProvider) SearchAlbums(ctx context.Context, query string) ([]model.SubsonicAlbum, error) {
	return a.itunes.SearchAlbums(ctx, query)
}

func (a *AggregatorProvider) GetAlbumSongs(ctx context.Context, albumID string) ([]model.SubsonicSong, error) {
	return a.itunes.GetAlbumSongs(ctx, albumID)
}

func (a *AggregatorProvider) GetSong(ctx context.Context, id string) (*model.SubsonicSong, error) {
	cleanID := strings.TrimPrefix(id, "external-")
	if val, ok := a.cache.Load("external-" + cleanID); ok {
		song := val.(model.SubsonicSong)
		return &song, nil
	}

	song, err := a.itunes.GetSong(ctx, cleanID)
	if err == nil && song != nil {
		return song, nil
	}

	if strings.HasPrefix(cleanID, "yt-") {
		ytID := strings.TrimPrefix(cleanID, "yt-")
		song := &model.SubsonicSong{
			ID:                    "external-" + cleanID,
			Title:                 ytID + " (external)",
			Artist:                "YouTube",
			DisplayArtist:         "YouTube",
			Album:                 "YouTube",
			AlbumID:               "external-yt-album",
			CoverArt:              "external-" + cleanID,
			Duration:              0,
			IsDir:                 false,
			ContentType:           "audio/mpeg",
			Suffix:                "mp3",
			TranscodedSuffix:      "mp3",
			TranscodedContentType: "audio/mpeg",
			Type:                  "music",
			MediaType:             "song",
			Created:               time.Now(),
			MusicBrainzId:         cleanID,
		}
		return song, nil
	}

	return nil, fmt.Errorf("song not found: %s", id)
}

func (a *AggregatorProvider) GetAlbum(ctx context.Context, id string) (*model.SubsonicAlbum, error) {
	return a.itunes.GetAlbum(ctx, id)
}

func (a *AggregatorProvider) GetCoverArt(ctx context.Context, id string, size int64) ([]byte, string, error) {
	cleanID := strings.TrimPrefix(id, "external-")
	externalID := "external-" + cleanID

	unescaped, err := url.QueryUnescape(cleanID)
	if err == nil && (strings.HasPrefix(unescaped, "http://") || strings.HasPrefix(unescaped, "https://")) {
		body, _, contentType, err := util.HTTPGet(ctx, unescaped, nil)
		if err == nil {
			return body, contentType, nil
		}
	}

	if coverURLVal, ok := a.coverMap.Load(externalID); ok {
		coverURL := coverURLVal.(string)
		if coverURL != "" {
			body, _, contentType, err := util.HTTPGet(ctx, coverURL, nil)
			if err == nil {
				return body, contentType, nil
			}
		}
	}

	if strings.HasPrefix(cleanID, "yt-") {
		ytID := strings.TrimPrefix(cleanID, "yt-")
		ytCoverURL := fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", ytID)
		body, _, contentType, err := util.HTTPGet(ctx, ytCoverURL, nil)
		if err == nil {
			return body, contentType, nil
		}
	}

	return a.itunes.GetCoverArt(ctx, cleanID, size)
}
