package metadata

import (
	"context"
	"time"

	"github.com/GerardPolloRebozado/navifetch/src/model"
	"github.com/GerardPolloRebozado/navifetch/src/util"
	"go.uploadedlobster.com/mbtypes"
	"go.uploadedlobster.com/musicbrainzws2"
)

type MusicBrainzProvider struct {
	client    *musicbrainzws2.Client
	paginator musicbrainzws2.Paginator
}

func (p *MusicBrainzProvider) GetAlbum(ctx context.Context, id string) (*model.SubsonicAlbum, error) {
	res, err := p.client.LookupReleaseGroup(ctx, mbtypes.MBID(id), musicbrainzws2.IncludesFilter{
		Includes: []string{"releases"},
	})
	if err != nil {
		return nil, err
	}
	album := MusicBrainzAlbumToSubsonicAlbum(res)
	return &album, nil
}

func NewMusicBrainzProvider(limit int) *MusicBrainzProvider {
	return &MusicBrainzProvider{musicbrainzws2.NewClient(musicbrainzws2.AppInfo{
		Name:    "navifetch",
		Version: "0.10.0",
		URL:     "https://github.com/GerardPolloRebozado/navifetch",
	}),
		musicbrainzws2.Paginator{
			Offset: 0,
			Limit:  limit,
		},
	}
}

func (p *MusicBrainzProvider) SearchSongs(ctx context.Context, query string) ([]model.SubsonicSong, error) {
	res, err := p.client.SearchRecordings(ctx, musicbrainzws2.SearchFilter{
		Query:  query,
		Dismax: true,
	},
		musicbrainzws2.Paginator{
			Offset: 0,
			Limit:  10,
		})
	if err != nil {
		return nil, err
	}
	songs := make([]model.SubsonicSong, 0)
	for _, song := range res.Recordings {
		songs = append(songs, MusicBrainzSongToSubsonicSong(song))
	}
	return songs, nil
}

func (p *MusicBrainzProvider) SearchAlbums(ctx context.Context, query string) ([]model.SubsonicAlbum, error) {
	res, err := p.client.SearchReleaseGroups(ctx, musicbrainzws2.SearchFilter{
		Query:  query,
		Dismax: true,
	},
		p.paginator,
	)
	if err != nil {
		return nil, err
	}
	albums := make([]model.SubsonicAlbum, 0)
	for _, album := range res.ReleaseGroups {
		albums = append(albums, MusicBrainzAlbumToSubsonicAlbum(album))
	}
	return albums, nil
}

func (p *MusicBrainzProvider) GetAlbumSongs(ctx context.Context, albumID string) ([]model.SubsonicSong, error) {
	res, err := p.client.LookupReleaseGroup(ctx, mbtypes.MBID(albumID), musicbrainzws2.IncludesFilter{
		Includes: []string{"releases"},
	})
	if err != nil {
		return nil, err
	}
	songs := make([]model.SubsonicSong, 0)
	for _, release := range res.Releases {
		for _, medium := range release.Media {
			for _, track := range medium.Tracks {
				songs = append(songs, MusicBrainzSongToSubsonicSong(track.Recording))
			}
		}
	}
	return songs, nil
}

func (p *MusicBrainzProvider) GetSong(ctx context.Context, id string) (*model.SubsonicSong, error) {
	includes := musicbrainzws2.IncludesFilter{
		Includes: []string{"releases", "artist-credits", "release-groups"},
	}
	res, err := p.client.LookupRecording(ctx, mbtypes.MBID(id), includes)
	if err != nil {
		return nil, err
	}
	song := MusicBrainzSongToSubsonicSong(res)
	return &song, nil
}

func (p *MusicBrainzProvider) GetCoverArt(ctx context.Context, id string, size int64) ([]byte, string, error) {
	url, err := GetCoverArtArchive(ctx, id, size)
	if err != nil {
		return nil, "", err
	}
	body, _, contentType, err := util.HTTPGet(ctx, url, nil)
	if err != nil {
		return nil, "", err
	}
	return body, contentType, nil
}

func MusicBrainzSongToSubsonicSong(recording musicbrainzws2.Recording) model.SubsonicSong {
	coverArt := recording.ID
	album := "Single"
	albumId := ""
	parent := string("external-" + recording.ID)
	if len(recording.Releases) > 0 {
		coverArt = recording.Releases[0].ID
		album = recording.Releases[0].ReleaseGroup.Title + " (external)"
		albumId = string("external-" + recording.Releases[0].ReleaseGroup.ID)
		parent = string("external-" + recording.Releases[0].ReleaseGroup.ID)
	}

	return model.SubsonicSong{
		ID:                 string("external-" + recording.ID),
		Parent:             parent,
		Title:              recording.Title,
		Artist:             recording.ArtistCredit.String(),
		DisplayArtist:      recording.ArtistCredit.String(),
		DisplayAlbumArtist: recording.ArtistCredit.String(),
		DisplayComposer:    "Display composer",
		ArtistID:           string("external-" + recording.ArtistCreditID),
		Album:              album,
		AlbumID:            albumId,
		Genre:              "",
		CoverArt:           string("external-" + coverArt),
		Duration:           int64(recording.Length.Duration.Seconds()),
		Size:               ((recording.Length.Duration.Milliseconds() / 1000) * 160000) / 8,
		IsDir:              false,
		IsVideo:            false,
		ContentType:        "song",
		Suffix:             "",
	}
}

func MusicBrainzAlbumToSubsonicAlbum(album musicbrainzws2.ReleaseGroup) model.SubsonicAlbum {
	return model.SubsonicAlbum{
		ID:        string("external-" + album.ID),
		Parent:    "",
		Album:     album.Title,
		Title:     album.Title,
		Name:      album.Title,
		IsDir:     true,
		CoverArt:  string("external-" + album.ID),
		SongCount: int64(len(album.Releases)),
		Created:   time.Now(),
		Duration:  1,
		PlayCount: 1,
		ArtistID:  string("external-" + album.ArtistCreditID),
		Artist:    album.ArtistCredit.String(),
		Year:      0,
		Genre:     "hello",
		Song:      []model.SubsonicSong{},
	}
}
