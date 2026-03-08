package model

import (
	"encoding/json"
	"time"
)

// ExternalItem represents an ephemeral search result.
type ExternalItem struct {
	Source        string  `json:"source"`
	RecordingID   string  `json:"recordingId,omitempty"`
	Title         string  `json:"title,omitempty"`
	ArtistID      int64   `json:"artistId,omitempty"`
	Artist        string  `json:"artist,omitempty"`
	Album         string  `json:"album,omitempty"`
	AlbumID       int64   `json:"albumId,omitempty"`
	Genre         string  `json:"genre,omitempty"`
	ReleaseID     string  `json:"releaseId,omitempty"`
	CoverArtURL   string  `json:"coverArtUrl,omitempty"`
	Confidence    float64 `json:"confidence,omitempty"`
	OriginalQuery string  `json:"originalQuery,omitempty"`
	Duration      int64   `json:"duration,omitempty"` // Seconds
	Size          int64   `json:"size,omitempty"`
}

// SubsonicSearchResponse is the top-level wrapper for Subsonic API responses
type SubsonicSearchResponse struct {
	Subsonic struct {
		Status        string         `json:"status"`
		Version       string         `json:"version"`
		SearchResult3 *SearchResult3 `json:"searchResult3,omitempty"`
		Song          []SubsonicSong `json:"song,omitempty"`
	} `json:"subsonic-response"`
}

type SubsonicAlbumResponse struct {
	Subsonic struct {
		Status  string         `json:"status"`
		Version string         `json:"version"`
		Album   *SubsonicAlbum `json:"album,omitempty"`
	} `json:"subsonic-response"`
}

type SubsonicAlbum struct {
	ID        string         `json:"id"`
	Parent    string         `json:"parent"`
	Album     string         `json:"album"`
	Title     string         `json:"title"`
	Name      string         `json:"name"`
	IsDir     bool           `json:"isDir"`
	CoverArt  string         `json:"coverArt"`
	SongCount int64          `json:"songCount"`
	Created   time.Time      `json:"created"`
	Duration  int            `json:"duration"`
	PlayCount int            `json:"playCount"`
	ArtistID  string         `json:"artistId"`
	Artist    string         `json:"artist"`
	Year      int            `json:"year"`
	Genre     string         `json:"genre"`
	Song      []SubsonicSong `json:"song"`
}

type SearchResult3 struct {
	Song []SubsonicSong `json:"song"`
}

func (s *SearchResult3) UnmarshalJSON(data []byte) error {
	type Alias SearchResult3
	var aux struct {
		Song any `json:"song"`
		*Alias
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Song == nil {
		return nil
	}
	switch aux.Song.(type) {
	case []any:
		// Re-unmarshal as slice of songs
		var songs struct {
			Song []SubsonicSong `json:"song"`
		}
		if err := json.Unmarshal(data, &songs); err == nil {
			s.Song = songs.Song
		}
	case map[string]any:
		// Single song object
		var single struct {
			Song SubsonicSong `json:"song"`
		}
		if err := json.Unmarshal(data, &single); err == nil {
			s.Song = []SubsonicSong{single.Song}
		}
	}
	return nil
}

type SubsonicSong struct {
	ID                    string    `json:"id"`
	Parent                string    `json:"parent,omitempty"`
	Title                 string    `json:"title"`
	Artist                string    `json:"artist"`
	ArtistID              string    `json:"artistId,omitempty"`
	Album                 string    `json:"album"`
	AlbumID               string    `json:"albumId,omitempty"`
	Genre                 string    `json:"genre,omitempty"`
	CoverArt              string    `json:"coverArt,omitempty"`
	Duration              int64     `json:"duration"`
	Size                  int64     `json:"size"`
	IsDir                 bool      `json:"isDir"`
	IsVideo               bool      `json:"isVideo"`
	Suffix                string    `json:"suffix"`
	ContentType           string    `json:"contentType"`
	TranscodedSuffix      string    `json:"transcodedSuffix,omitempty"`
	TranscodedContentType string    `json:"transcodedContentType,omitempty"`
	Type                  string    `json:"type,omitempty"`
	MediaType             string    `json:"mediaType,omitempty"`
	Created               time.Time `json:"created,omitempty"`
	Path                  string    `json:"path,omitempty"`
	ChannelCount          int       `json:"channelCount,omitempty"`
	BitDepth              int       `json:"bitDepth,omitempty"`
	SamplingRate          int       `json:"samplingRate,omitempty"`
	Bpm                   int       `json:"bpm,omitempty"`
	Comment               string    `json:"comment,omitempty"`
	SortName              string    `json:"sortName,omitempty"`
	MusicBrainzId         string    `json:"musicBrainzId,omitempty"`
	DisplayArtist         string    `json:"displayArtist,omitempty"`
	DisplayAlbumArtist    string    `json:"displayAlbumArtist,omitempty"`
	DisplayComposer       string    `json:"displayComposer,omitempty"`
	ExplicitStatus        string    `json:"explicitStatus,omitempty"`
}

type SubsonicIndexResponse struct {
	Subsonic struct {
		Status        string         `json:"status"`
		Version       string         `json:"version"`
		SearchResult3 *SearchResult3 `json:"searchResult3,omitempty"`
		Indexes       struct {
			Index []struct {
				Name   string `json:"name"`
				Artist []struct {
					ID             string `json:"id"`
					Name           string `json:"name"`
					CoverArt       string `json:"coverArt"`
					ArtistImageURL string `json:"artistImageUrl"`
				} `json:"artist"`
			} `json:"index"`
			LastModified    int64  `json:"lastModified"`
			IgnoredArticles string `json:"ignoredArticles"`
		} `json:"indexes"`
	} `json:"subsonic-response"`
}
