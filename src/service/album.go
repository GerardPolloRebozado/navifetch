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

		// Try to use MBID if Navidrome has it
		if subsonicAlbumResponse.Subsonic.Album.MusicBrainzId != "" {
			log.Printf("Enriching local album using MBID: %s", subsonicAlbumResponse.Subsonic.Album.MusicBrainzId)
			externalSongs, err = s.metadata.GetAlbumSongs(ctx, subsonicAlbumResponse.Subsonic.Album.MusicBrainzId)
		}

		if len(externalSongs) == 0 {
			albumName := subsonicAlbumResponse.Subsonic.Album.Name
			if albumName == "" {
				albumName = subsonicAlbumResponse.Subsonic.Album.Album
			}
			if albumName == "" {
				albumName = subsonicAlbumResponse.Subsonic.Album.Title
			}
			artistName := subsonicAlbumResponse.Subsonic.Album.Artist
			searchQuery := albumName
			if artistName != "" {
				searchQuery = artistName + " - " + albumName
			}
			log.Printf("Enriching local album via search. ID: %s, Name: '%s', Album: '%s', Title: '%s', Artist: '%s'. Query: '%s'",
				albumID, subsonicAlbumResponse.Subsonic.Album.Name, subsonicAlbumResponse.Subsonic.Album.Album,
				subsonicAlbumResponse.Subsonic.Album.Title, artistName, searchQuery)

			externalAlbums, err := s.metadata.SearchAlbums(ctx, searchQuery)
			if err == nil && len(externalAlbums) > 0 {
				log.Printf("Found %d external albums for query '%s'. Taking first: %s", len(externalAlbums), searchQuery, externalAlbums[0].ID)
				externalSongs, err = s.metadata.GetAlbumSongs(ctx, externalAlbums[0].ID)
				if err != nil {
					log.Printf("Error fetching songs for external album %s: %v", externalAlbums[0].ID, err)
				}
			} else if err != nil {
				log.Printf("Error searching external albums for query '%s': %v", searchQuery, err)
			} else {
				log.Printf("No external albums found for query '%s'", searchQuery)
			}
		}
	}

	log.Printf("External songs found: %d for album %s", len(externalSongs), albumID)
	// Merge logic (common to both if external songs were found)
	if subsonicAlbumResponse.Subsonic.Album != nil && len(externalSongs) > 0 {
		existingSongs := subsonicAlbumResponse.Subsonic.Album.Song
		log.Printf("Local album has %d songs. Merging with %d external songs.", len(existingSongs), len(externalSongs))
		for _, song := range externalSongs {
			if !util.IsSongInSubsonicSongList(strings.TrimSpace(song.Title), existingSongs) {
				log.Printf("Adding missing song to album: %s", song.Title)
				subsonicAlbumResponse.Subsonic.Album.Song = append(subsonicAlbumResponse.Subsonic.Album.Song, song)
			}
		}
		newCount := len(subsonicAlbumResponse.Subsonic.Album.Song)
		log.Printf("Album %s enrichment complete: %d -> %d songs", albumID, len(existingSongs), newCount)
		subsonicAlbumResponse.Subsonic.Album.SongCount = int64(newCount)
	}

	return &subsonicAlbumResponse, nil
}
