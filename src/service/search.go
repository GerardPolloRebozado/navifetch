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
	subsonicSongs, _, err := s.rp.SearchNavidrome(ctx, path, rawQuery)

	if len(subsonicSongs) > 0 {
		jsonBody, err := json.Marshal(WrapExternalSearch(subsonicSongs))
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

func WrapExternalSearch(songs []model.SubsonicSong) any {
	return model.SubsonicResponseWrapper{
		SubsonicResponse: model.SubsonicSearchResponseBody{
			Status:  "ok",
			Version: "1.16.1",
			SearchResult3: model.SearchResult3{
				Song:   songs,
				Album:  []map[string]any{},
				Artist: []map[string]any{},
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
