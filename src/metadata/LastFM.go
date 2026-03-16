package metadata

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/GerardPolloRebozado/navifetch/src/model"
	"github.com/GerardPolloRebozado/navifetch/src/util"
	"github.com/twoscott/gobble-fm/api"
	"github.com/twoscott/gobble-fm/lastfm"
)

type LastFMProvider struct {
	client *api.Client
	limit  int
}

func NewLastFMProvider(apiKey string, limit int) *LastFMProvider {
	return &LastFMProvider{
		client: api.NewClientKeyOnly(apiKey),
		limit:  limit,
	}
}

func (p *LastFMProvider) SearchSongs(_ context.Context, query string) ([]model.SubsonicSong, error) {
	params := lastfm.TrackSearchParams{
		Track: query,
		Limit: uint(p.limit),
	}
	res, err := p.client.Track.Search(params)
	if err != nil {
		return nil, err
	}

	songs := make([]model.SubsonicSong, 0)
	for _, t := range res.Tracks {
		songs = append(songs, p.toSubsonicSong(t.Title, t.Artist, t.MBID, "", "", 0))
	}
	return songs, nil
}

func (p *LastFMProvider) SearchAlbums(_ context.Context, query string) ([]model.SubsonicAlbum, error) {
	params := lastfm.AlbumSearchParams{
		Album: query,
		Limit: uint(p.limit),
	}
	res, err := p.client.Album.Search(params)
	if err != nil {
		return nil, err
	}

	albums := make([]model.SubsonicAlbum, 0)
	for _, a := range res.Albums {
		mbid := a.MBID
		if mbid == "" && a.Artist != "" && a.Title != "" {
			info, err := p.client.Album.Info(lastfm.AlbumInfoParams{
				Artist: a.Artist,
				Album:  a.Title,
			})
			if err == nil && info != nil {
				mbid = info.MBID
			}
		}

		if mbid != "" {
			albums = append(albums, p.toSubsonicAlbum(a.Title, a.Artist, mbid, 0))
		}
	}
	return albums, nil
}

func (p *LastFMProvider) GetAlbumSongs(_ context.Context, albumID string) ([]model.SubsonicSong, error) {
	albumID = strings.TrimPrefix(albumID, "external-")
	albumRes, err := p.client.Album.InfoByMBID(lastfm.AlbumInfoMBIDParams{MBID: albumID})
	if err != nil {
		return nil, err
	}

	songs := make([]model.SubsonicSong, len(albumRes.Tracks))
	var wg sync.WaitGroup
	for i, t := range albumRes.Tracks {
		wg.Add(1)
		go func(idx int, trackTitle, trackArtist string) {
			defer wg.Done()
			trackRes, err := p.client.Track.Info(lastfm.TrackInfoParams{
				Artist: trackArtist,
				Track:  trackTitle,
			})
			if err == nil && trackRes != nil {
				songs[idx] = p.toSubsonicSong(trackRes.Title, trackRes.Artist.Name, trackRes.MBID, trackRes.Album.Title, trackRes.Album.MBID, int64(trackRes.Duration.Unwrap().Seconds()))
			} else {
				songs[idx] = p.toSubsonicSong(trackTitle, trackArtist, "", albumRes.Title, albumRes.MBID, 0)
			}
		}(i, t.Title, t.Artist.Name)
	}
	wg.Wait()

	return songs, nil
}

func (p *LastFMProvider) GetSong(_ context.Context, id string) (*model.SubsonicSong, error) {
	id = strings.TrimPrefix(id, "external-")
	res, err := p.client.Track.InfoByMBID(lastfm.TrackInfoMBIDParams{MBID: id})
	if err != nil {
		return nil, err
	}

	song := p.toSubsonicSong(res.Title, res.Artist.Name, res.MBID, res.Album.Title, res.Album.MBID, int64(res.Duration.Unwrap().Seconds()))
	return &song, nil
}

func (p *LastFMProvider) GetAlbum(_ context.Context, id string) (*model.SubsonicAlbum, error) {
	id = strings.TrimPrefix(id, "external-")
	res, err := p.client.Album.InfoByMBID(lastfm.AlbumInfoMBIDParams{MBID: id})
	if err != nil {
		return nil, err
	}

	a := p.toSubsonicAlbum(res.Title, res.Artist, res.MBID, int64(len(res.Tracks)))
	return &a, nil
}

func (p *LastFMProvider) GetCoverArt(ctx context.Context, id string, _ int64) ([]byte, string, error) {
	id = strings.TrimPrefix(id, "external-")
	var imageURL string
	tInfo, err := p.client.Track.InfoByMBID(lastfm.TrackInfoMBIDParams{MBID: id})
	if err == nil && tInfo != nil && tInfo.Album.Image.OriginalURL() != "" {
		imageURL = tInfo.Album.Image.OriginalURL()
	} else {
		aInfo, err := p.client.Album.InfoByMBID(lastfm.AlbumInfoMBIDParams{MBID: id})
		if err == nil && aInfo != nil && aInfo.Image.OriginalURL() != "" {
			imageURL = aInfo.Image.OriginalURL()
		}
	}

	if imageURL == "" {
		return nil, "", fmt.Errorf("no cover art found for ID: %s", id)
	}

	body, _, contentType, err := util.HTTPGet(ctx, imageURL, nil)
	return body, contentType, err
}

func (p *LastFMProvider) toSubsonicSong(title, artist, mbid, album, albumMBID string, duration int64) model.SubsonicSong {
	coverArtID := albumMBID
	if coverArtID == "" {
		coverArtID = mbid
	}

	return model.SubsonicSong{
		ID:                    "external-" + mbid,
		Title:                 title + " (external)",
		Artist:                artist,
		DisplayArtist:         artist,
		Album:                 album,
		AlbumID:               "external-" + albumMBID,
		CoverArt:              "external-" + coverArtID,
		Duration:              duration,
		IsDir:                 false,
		ContentType:           "audio/mpeg",
		Suffix:                "mp3",
		TranscodedSuffix:      "mp3",
		TranscodedContentType: "audio/mpeg",
		Type:                  "music",
		MediaType:             "song",
		Created:               time.Now(),
		MusicBrainzId:         mbid,
	}
}

func (p *LastFMProvider) toSubsonicAlbum(title, artist, mbid string, songCount int64) model.SubsonicAlbum {
	return model.SubsonicAlbum{
		ID:        "external-" + mbid,
		Album:     title,
		Title:     title,
		Name:      title,
		Artist:    artist,
		CoverArt:  "external-" + mbid,
		SongCount: songCount,
		IsDir:     true,
		Created:   time.Now(),
	}
}
