package service

import (
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/GerardPolloRebozado/navifetch/src/config"
)

func StartCleanupCron(cfg *config.Config) {
	go UpdateYTDLP(cfg)

	ticker := time.NewTicker(24 * time.Hour)

	go func() {
		for range ticker.C {
			CleanupJob(cfg)
			UpdateYTDLP(cfg)
		}
	}()
}

func UpdateYTDLP(cfg *config.Config) {
	slog.Info("Checking for yt-dlp self-update...")
	cmd := exec.Command(cfg.YTDLPPath, "-U")
	output, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(output))
	if err != nil {
		slog.Warn("yt-dlp update output", "status", outStr, "error", err)
		return
	}
	slog.Info("yt-dlp update status", "output", outStr)
}

func IsFolderEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func CleanFile(path string) {
	files, err := os.ReadDir(path)
	if err != nil {
		slog.Error("Error reading dir: " + path)
		return
	}
	for _, file := range files {
		currFilePath := path + "/" + file.Name()
		if file.IsDir() {
			isEmpty, err := IsFolderEmpty(currFilePath)
			if isEmpty || err != nil {
				os.Remove(currFilePath)
				continue
			}
			CleanFile(currFilePath)
			continue
		}
		meta, err := file.Info()
		if err != nil {
			slog.Error("Error reading file info: " + currFilePath)
			continue
		}
		if time.Now().Unix()-meta.ModTime().Unix() > 86400 {
			os.Remove(currFilePath)
		}
	}
}

func CleanupJob(cfg *config.Config) {
	slog.Info("Running cleanup job")

	CleanFile(cfg.MusicLibraryPath + "/" + "cached")

	slog.Info("Cleanup job finished.")
}
