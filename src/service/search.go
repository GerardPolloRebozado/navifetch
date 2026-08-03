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
	subsonicSongs, _, _ := s.rp.SearchNavidrome(ctx, path, rawQuery)

	externalSongs, err := s.metadata.SearchSongs(ctx, query)
	if err != nil {
		externalSongs = nil
	}

	allSongs := append(subsonicSongs, externalSongs...)

	resp := WrapExternalSearch(allSongs)
	jsonBody, err := json.Marshal(resp)
	if err != nil {
		return nil, "", err
	}

	return jsonBody, "application/json; charset=utf-8", nil
}

func WrapExternalSearch(songs []model.SubsonicSong) any {
	if songs == nil {
		songs = []model.SubsonicSong{}
	}
	sr := model.SearchResult3{
		Song:   songs,
		Album:  []map[string]any{},
		Artist: []map[string]any{},
	}
	return map[string]any{
		"subsonic-response": map[string]any{
			"status":        "ok",
			"version":       "1.16.1",
			"type":          "navidrome",
			"serverVersion": "0.52.5",
			"openSubsonic":  true,
			"searchResult3": sr,
			"searchResult2": sr,
			"searchResult":  sr,
		},
	}
}

func ContentTypeOrJSON(ct string) string {
	if ct != "" {
		return ct
	}
	return "application/json; charset=utf-8"
}
