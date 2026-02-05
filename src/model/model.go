package model

import "time"

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
		Song          *SubsonicSong  `json:"song,omitempty"`
	} `json:"subsonic-response"`
}

type SubsonicAlbumResponse struct {
	Subsonic struct {
		Status  string `json:"status"`
		Version string `json:"version"`
		Album   *Album `json:"album,omitempty"`
	} `json:"subsonic-response"`
}

type Album struct {
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
	Song any `json:"song"`
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
	Size                  int64     `json:"size,omitempty"`
	IsDir                 bool      `json:"isDir"`
	IsVideo               bool      `json:"isVideo"`
	Suffix                string    `json:"suffix,omitempty"`
	ContentType           string    `json:"contentType,omitempty"`
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
	DisplayAlbumArtists   string    `json:"displayAlbumArtists,omitempty"`
	DisplayComposer       string    `json:"displayComposer,omitempty"`
	ExplicitStatus        string    `json:"explicitStatus,omitempty"`
}

type ITunesAlbum struct {
	WrapperType            string    `json:"wrapperType"`
	CollectionType         string    `json:"collectionType"`
	ArtistID               int       `json:"artistId"`
	CollectionID           int       `json:"collectionId"`
	AmgArtistID            int       `json:"amgArtistId"`
	ArtistName             string    `json:"artistName"`
	CollectionName         string    `json:"collectionName"`
	CollectionCensoredName string    `json:"collectionCensoredName"`
	ArtistViewURL          string    `json:"artistViewUrl"`
	CollectionViewURL      string    `json:"collectionViewUrl"`
	ArtworkURL60           string    `json:"artworkUrl60"`
	ArtworkURL100          string    `json:"artworkUrl100"`
	CollectionPrice        float64   `json:"collectionPrice"`
	CollectionExplicitness string    `json:"collectionExplicitness"`
	TrackCount             int       `json:"trackCount"`
	Copyright              string    `json:"copyright"`
	Country                string    `json:"country"`
	Currency               string    `json:"currency"`
	ReleaseDate            time.Time `json:"releaseDate"`
	PrimaryGenreName       string    `json:"primaryGenreName"`
}
