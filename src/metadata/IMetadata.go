package metadata

import (
	"context"
	"fmt"

	"github.com/GerardPolloRebozado/navifetch/src/model"
)

type Provider interface {
	SearchSongs(ctx context.Context, query string) ([]model.SubsonicSong, error)
	SearchAlbums(ctx context.Context, query string) ([]model.SubsonicAlbum, error)
	GetAlbumSongs(ctx context.Context, albumID string) ([]model.SubsonicSong, error)
	GetSong(ctx context.Context, id string) (*model.SubsonicSong, error)
	GetAlbum(ctx context.Context, id string) (*model.SubsonicAlbum, error)
	GetCoverArt(ctx context.Context, id string, size int64) ([]byte, string, error)
}

var metadataProvider Provider

func NewProvider(name string, country string, limit int) (Provider, error) {
	if metadataProvider != nil {
		return metadataProvider, nil
	}
	switch name {
	case "itunes":
		metadataProvider = NewItunesProvider(country, limit)
		return metadataProvider, nil
	case "musicbrainz":
		metadataProvider = NewMusicBrainzProvider(limit)
		return metadataProvider, nil
	default:
		return nil, fmt.Errorf("unsupported metadata provider: %s", name)
	}
}

func GetProvider() Provider {
	if metadataProvider == nil {
		panic("metadata provider not initialized")
	}
	return metadataProvider
}
