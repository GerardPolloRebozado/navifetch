package service

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/GerardPolloRebozado/navifetch/src/metadata"
	"github.com/GerardPolloRebozado/navifetch/src/model"
	"github.com/GerardPolloRebozado/navifetch/src/util"
)

type AlbumService struct {
	upstream NavidromeClient
	metadata metadata.Provider
}

func NewAlbumService(upstream NavidromeClient, metadata metadata.Provider) *AlbumService {
	return &AlbumService{
		upstream: upstream,
		metadata: metadata,
	}
}

func (s *AlbumService) GetAlbum(ctx context.Context, albumID string, path string, rawQuery string) (*model.SubsonicAlbumResponse, error) {
	isAlbumExternal := strings.HasPrefix(albumID, "external-")
	albumTrimmedID := strings.TrimPrefix(albumID, "external-")
	var subsonicAlbumResponse model.SubsonicAlbumResponse
	var externalSongs []model.SubsonicSong

	if isAlbumExternal {
		album, err := s.metadata.GetAlbum(ctx, albumTrimmedID)
		if err != nil {
			return nil, err
		}

		songs, err := s.metadata.GetAlbumSongs(ctx, albumTrimmedID)
		if err != nil {
			log.Printf("Error fetching external album songs %s: %v", albumID, err)
		}

		subsonicAlbumResponse.Subsonic.Status = "ok"
		subsonicAlbumResponse.Subsonic.Version = "1.16.1"
		subsonicAlbumResponse.Subsonic.Album = album
		subsonicAlbumResponse.Subsonic.Album.Song = songs
	} else {
		// Local album: Fetch from Navidrome
		body, _, _, err := s.upstream.SendNavidromeRequest(ctx, path, rawQuery)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(body, &subsonicAlbumResponse); err != nil {
			return nil, err
		}

		if subsonicAlbumResponse.Subsonic.Album == nil {
			return &subsonicAlbumResponse, nil
		}

		// Enrich local album with external songs if possible
		externalAlbums, err := s.metadata.SearchAlbums(ctx, subsonicAlbumResponse.Subsonic.Album.Name)
		if err == nil && len(externalAlbums) > 0 {
			externalSongs, _ = s.metadata.GetAlbumSongs(ctx, externalAlbums[0].ID)
		}
	}

	// Merge logic (common to both if external songs were found)
	if subsonicAlbumResponse.Subsonic.Album != nil && len(externalSongs) > 0 {
		for _, song := range externalSongs {
			if !util.IsSongInSubsonicSongList(song.Title, subsonicAlbumResponse.Subsonic.Album.Song) {
				subsonicAlbumResponse.Subsonic.Album.Song = append(subsonicAlbumResponse.Subsonic.Album.Song, song)
			}
		}
		subsonicAlbumResponse.Subsonic.Album.SongCount = int64(len(subsonicAlbumResponse.Subsonic.Album.Song))
	}

	return &subsonicAlbumResponse, nil
}
