package service

import (
	"context"
	"encoding/json"

	"github.com/GerardPolloRebozado/navifetch/src/metadata"
	"github.com/GerardPolloRebozado/navifetch/src/model"
)

type SearchService struct {
	rp       *SubsonicReverseProxy
	metadata metadata.Provider
}

func NewSearchService(rp *SubsonicReverseProxy, metadata metadata.Provider) *SearchService {
	return &SearchService{
		rp:       rp,
		metadata: metadata,
	}
}

func (s *SearchService) SmartSearch(ctx context.Context, query string, path string, rawQuery string) ([]byte, string, error) {
	body, contentType, err := s.rp.SearchNavidrome(ctx, path, rawQuery)
	if err == nil && body != nil {
		bytes, err := json.Marshal(body)
		if err != nil {
			return nil, "", err
		}
		return bytes, contentType, nil
	}

	if len(body) > 0 {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, "", err
		}

		return jsonBody, "application/json; charset=utf-8", nil
	}

	songs, err := s.metadata.SearchSongs(ctx, query)
	if err != nil {
		return nil, "", err
	}

	resp := WrapExternalSearch(songs)
	jsonBody, err := json.Marshal(resp)
	if err != nil {
		return nil, "", err
	}

	return jsonBody, "application/json; charset=utf-8", nil
}

func WrapExternalSearch(songs []model.SubsonicSong) map[string]any {
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
