package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/GerardPolloRebozado/navifetch/src/config"
	"github.com/GerardPolloRebozado/navifetch/src/metadata"
	"github.com/GerardPolloRebozado/navifetch/src/model"
	"github.com/GerardPolloRebozado/navifetch/src/util"
)

type StreamService struct {
	cfg      *config.Config
	metadata metadata.Provider
}

func NewStreamService(cfg *config.Config, metadata metadata.Provider) *StreamService {
	return &StreamService{
		cfg:      cfg,
		metadata: metadata,
	}
}

func (s *StreamService) DownloadTrack(trackID string, permanent bool) (*model.SubsonicSong, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	res, err := s.metadata.GetSong(ctx, trackID)
	if err != nil {
		log.Printf("Background download failed lookup for %s: %v", trackID, err)
		return nil, "", err
	}

	artist := strings.TrimSuffix(res.Artist, "(external)")
	album := strings.TrimSuffix(res.Album, "(external)")
	title := strings.TrimSuffix(res.Title, "(external)")
	coverURL := res.CoverArt
	targetPath := util.GetTrackPath(s.cfg, artist, album, title, permanent)
	if _, err := os.Stat(targetPath); err == nil {
		return res, targetPath, nil
	}

	log.Printf("Saving permanent copy for Navidrome: %s", targetPath)
	_ = os.MkdirAll(filepath.Dir(targetPath), 0755)

	coverPath := ""
	if coverURL != "" {
		tmpCover, err := os.CreateTemp("", "cover-*.jpg")
		if err == nil {
			defer os.Remove(tmpCover.Name())
			defer tmpCover.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			body, status, _, err := util.HTTPGet(ctx, coverURL, nil)
			if err == nil && status == 200 {
				_, _ = tmpCover.Write(body)
				coverPath = tmpCover.Name()
			}
		}
	}

	searchQuery := fmt.Sprintf("ytsearch1:%s - %s Audio", artist, title)

	safeTitle := strings.ReplaceAll(title, "'", "")
	safeArtist := strings.ReplaceAll(artist, "'", "")
	safeAlbum := strings.ReplaceAll(album, "'", "")

	ffmpegArgs := fmt.Sprintf("ffmpeg:-metadata title='%s' -metadata artist='%s' -metadata album='%s'", safeTitle, safeArtist, safeAlbum)

	cleanMBID := trackID
	if strings.HasPrefix(cleanMBID, "external-") {
		cleanMBID = strings.TrimPrefix(cleanMBID, "external-")
	}

	args := []string{
		"-x", "--audio-format", "mp3",
		"--postprocessor-args", ffmpegArgs,
		"-o", targetPath,
		"--no-playlist",
		"--add-metadata",
		"--postprocessor-args",
		"Metadata:-metadata musicbrainz_trackid=" + cleanMBID,
	}
	if coverPath != "" {
		args = append(args, "--embed-thumbnail")
	} else {
		args = append(args, "--embed-thumbnail")
	}

	args = append(args, searchQuery)

	cmd := exec.Command(s.cfg.YTDLPPath, args...)

	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Failed to save permanent copy: %v\nOutput: %s", err, string(output))
		return nil, "", err
	}

	log.Printf("Successfully saved: %s", targetPath)
	return res, targetPath, nil
}
