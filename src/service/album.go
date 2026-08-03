package service

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/GerardPolloRebozado/navifetch/src/metadata"
	"github.com/GerardPolloRebozado/navifetch/src/model"
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
		body, _, _, err := s.upstream.SendNavidromeRequest(ctx, path, rawQuery)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(body, &subsonicAlbumResponse); err != nil {
			return nil, err
		}

		return &subsonicAlbumResponse, nil
	}

	return &subsonicAlbumResponse, nil
}
