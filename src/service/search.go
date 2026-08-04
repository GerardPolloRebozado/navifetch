package service

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"

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

	u, _ := url.ParseQuery(rawQuery)
	format := u.Get("f")

	if format == "xml" || format == "" {
		xmlBody := WrapExternalSearchXML(allSongs)
		return xmlBody, "text/xml; charset=utf-8", nil
	}

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

func WrapExternalSearchXML(songs []model.SubsonicSong) []byte {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString(`<subsonic-response xmlns="http://subsonic.org/restapi" status="ok" version="1.16.1" type="navidrome" serverVersion="0.52.5" openSubsonic="true">`)
	sb.WriteString(`<searchResult3>`)
	for _, s := range songs {
		sb.WriteString(fmt.Sprintf(`<song id="%s" title="%s" artist="%s" album="%s" duration="%d" coverArt="%s" isDir="false" isVideo="false" suffix="mp3" contentType="audio/mpeg"/>`,
			xmlEscape(s.ID), xmlEscape(s.Title), xmlEscape(s.Artist), xmlEscape(s.Album), s.Duration, xmlEscape(s.CoverArt)))
	}
	sb.WriteString(`</searchResult3>`)
	sb.WriteString(`<searchResult2>`)
	for _, s := range songs {
		sb.WriteString(fmt.Sprintf(`<song id="%s" title="%s" artist="%s" album="%s" duration="%d" coverArt="%s" isDir="false" isVideo="false" suffix="mp3" contentType="audio/mpeg"/>`,
			xmlEscape(s.ID), xmlEscape(s.Title), xmlEscape(s.Artist), xmlEscape(s.Album), s.Duration, xmlEscape(s.CoverArt)))
	}
	sb.WriteString(`</searchResult2>`)
	sb.WriteString(`</subsonic-response>`)
	return []byte(sb.String())
}

func xmlEscape(s string) string {
	var buf strings.Builder
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func ContentTypeOrJSON(ct string) string {
	if ct != "" {
		return ct
	}
	return "application/json; charset=utf-8"
}
