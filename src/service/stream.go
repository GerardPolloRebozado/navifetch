package service

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/GerardPolloRebozado/navitube/src/client"
	"github.com/GerardPolloRebozado/navitube/src/config"
	"github.com/GerardPolloRebozado/navitube/src/util"
)

func DownloadTrack(cfg *config.Config, artist, album, title, targetPath, coverURL string, authParams url.Values) {
	if _, err := os.Stat(targetPath); err == nil {
		return
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

	args := []string{
		"-x", "--audio-format", "mp3",
		"--add-metadata",
		"--postprocessor-args", ffmpegArgs,
		"-o", targetPath,
		"--no-playlist",
	}
	if coverPath != "" {
		args = append(args, "--embed-thumbnail")
	} else {
		args = append(args, "--embed-thumbnail")
	}

	args = append(args, searchQuery)

	cmd := exec.Command(cfg.YTDLPPath, args...)

	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("Failed to save permanent copy: %v\nOutput: %s", err, string(output))
	} else {
		log.Printf("Successfully saved: %s", targetPath)
		client.TriggerNavidromeScan(cfg.NavidromeBase, authParams)
	}
}
