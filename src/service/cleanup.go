package service

import (
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/GerardPolloRebozado/navitube/src/config"
)

func StartCleanupCron(cfg *config.Config) {
	ticker := time.NewTicker(24 * time.Hour)

	go func() {
		for range ticker.C {
			CleanupJob(cfg)
		}
	}()
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
