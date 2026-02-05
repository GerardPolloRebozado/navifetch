package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/GerardPolloRebozado/navitube/src/client"
	"github.com/GerardPolloRebozado/navitube/src/model"
	"github.com/torabit/itunes"
)

func ItunesSongToSubsonicSong(rec itunes.Result) model.SubsonicSong {
	return model.SubsonicSong{
		Parent:                rec.CollectionName,
		ID:                    fmt.Sprintf("itunes-%d", rec.TrackId),
		Title:                 rec.TrackName + " iTunes",
		Artist:                rec.ArtistName,
		ArtistID:              fmt.Sprintf("itunes-%d", rec.ArtistId),
		Album:                 rec.CollectionName,
		AlbumID:               fmt.Sprintf("itunes-%d", rec.CollectionId),
		Genre:                 rec.PrimaryGenreName,
		CoverArt:              "itunes-cover-" + url.QueryEscape(rec.ArtworkUrl100),
		Duration:              rec.TrackTimeMillis / 1000,
		Size:                  ((rec.TrackTimeMillis / 1000) * 160000) / 8,
		IsDir:                 false,
		IsVideo:               false,
		Suffix:                "mp3",
		ContentType:           "audio/mpeg",
		TranscodedSuffix:      "mp3",
		TranscodedContentType: "audio/mpeg",
		Type:                  "music",
		MediaType:             "song",
		Created:               time.Now(),
		ChannelCount:          2,
		BitDepth:              16,
		SamplingRate:          44100,
		Bpm:                   1,
		Comment:               "itunes",
		SortName:              rec.TrackName,
		MusicBrainzId:         "",
		DisplayArtist:         rec.ArtistName,
		DisplayAlbumArtists:   rec.ArtistName,
		DisplayComposer:       rec.ArtistName,
		ExplicitStatus:        "clean",
	}
}

func ItunesAlbumToSubsonicAlbum(rec itunes.Result) model.Album {
	return model.Album{
		ID:        fmt.Sprintf("itunes-%d", rec.CollectionId),
		Parent:    fmt.Sprintf("itunes-%d", rec.ArtistId),
		Album:     rec.CollectionName,
		Title:     rec.CollectionName,
		Name:      rec.CollectionName,
		IsDir:     true,
		CoverArt:  "itunes-cover-" + url.QueryEscape(rec.ArtworkUrl100),
		SongCount: int64(rec.TrackCount),
		Created:   time.Now(),
		ArtistID:  fmt.Sprintf("itunes-%d", rec.ArtistId),
		Artist:    rec.ArtistName,
		Genre:     rec.PrimaryGenreName,
	}
}

func PerformSmartSearch(ctx context.Context, query string) ([]itunes.Result, error) {
	itunesClient := client.GetItunesClient()
	res, err := itunesClient.Search(ctx,
		itunes.Term(query),
		itunes.Media(itunes.MediaMusic))

	if err != nil {
		return nil, err
	}

	return res.Results, nil
}

func PerformAlbumSearch(ctx context.Context, query string) ([]itunes.Result, error) {
	itunesClient := client.GetItunesClient()
	res, err := itunesClient.Search(ctx,
		itunes.Term(query),
		itunes.Media(itunes.MediaMusic),
		itunes.Entity(itunes.EntityAlbum))

	if err != nil {
		return nil, err
	}

	return res.Results, nil
}

func PerformAlbumSongSearch(ctx context.Context, id int64) ([]itunes.Result, error) {
	itunesClient := client.GetItunesClient()
	res, err := itunesClient.Lookup(ctx, itunes.ID(id),
		itunes.Entity(itunes.EntitySong),
		itunes.Media(itunes.MediaMusic))
	if err != nil {
		return nil, err
	}
	filtered := res.Results[:0]
	for _, r := range res.Results {
		if r.WrapperType == "track" {
			filtered = append(filtered, r)
		}
	}
	res.Results = filtered

	if len(res.Results) == 0 {
		return nil, errors.New("no results found")
	}
	return res.Results, nil
}

func WrapExternalSearch(results []itunes.Result) map[string]any {
	songs := make([]model.SubsonicSong, len(results))
	for i, rec := range results {
		songs[i] = ItunesSongToSubsonicSong(rec)
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

func ContentTypeOrJSON(ct string) string {
	if ct != "" {
		return ct
	}
	return "application/json; charset=utf-8"
}
