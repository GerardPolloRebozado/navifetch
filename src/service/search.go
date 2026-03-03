package service

import (
	"context"
	"encoding/json"

	"github.com/GerardPolloRebozado/navifetch/src/metadata"
	"github.com/GerardPolloRebozado/navifetch/src/model"
)

type SearchService struct {
	upstream NavidromeClient
	metadata metadata.Provider
}

func NewSearchService(upstream NavidromeClient, metadata metadata.Provider) *SearchService {
	return &SearchService{
		upstream: upstream,
		metadata: metadata,
	}
}

func (s *SearchService) SmartSearch(ctx context.Context, query string, path string, rawQuery string) ([]byte, string, error) {
	body, contentType, err := s.upstream.SearchNavidrome(ctx, path, rawQuery)
	if err == nil && body != nil {
		bytes, err := json.Marshal(body)
		if err != nil {
			return nil, "", err
		}
		return bytes, contentType, nil
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
