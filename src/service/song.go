package service

import (
	"context"
	"log"

	"github.com/GerardPolloRebozado/navifetch/src/metadata"
	"github.com/GerardPolloRebozado/navifetch/src/model"
)

type SongService struct {
	upstream NavidromeClient
	metadata metadata.Provider
}

func NewSongService(upstream NavidromeClient, metadata metadata.Provider) *SongService {
	return &SongService{
		upstream: upstream,
		metadata: metadata,
	}
}

func (s *SongService) GetSong(ctx context.Context, id string) (*model.SubsonicSong, error) {
	return s.metadata.GetSong(ctx, id)
}

func (s *SongService) GetCoverArt(ctx context.Context, id string, size int64) ([]byte, string, error) {
	image, contentType, err := s.metadata.GetCoverArt(ctx, id, size)
	if err != nil {
		log.Printf("Error fetching cover art: %v", err)
		return nil, "", err
	}
	return image, contentType, nil
}
