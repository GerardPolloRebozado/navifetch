package main

import (
	"io"
	"log/slog"
	"os"
	"time"
)

func StartCleanupCron(cfg *Config) {
	ticker := time.NewTicker(24 * time.Hour)

	go func() {
		for {
			select {
			case <-ticker.C:
				cleanupJob(cfg)
			}
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

func cleanFile(path string) {
	files, err := os.ReadDir(path)
	if err != nil {
		slog.Error("Error reading dir: " + path)
	}
	for _, file := range files {
		currFilePath := path + "/" + file.Name()
		if file.IsDir() {
			isEmpty, err := IsFolderEmpty(currFilePath)
			if isEmpty || err != nil {
				os.Remove(currFilePath)
				continue
			}
			cleanFile(currFilePath)
			continue
		}
		meta, err := file.Info()
		if err != nil {
			slog.Error("Error reading file info: " + currFilePath)
		}
		if time.Now().Unix()-meta.ModTime().Unix() > 86400 {
			os.Remove(currFilePath)
		}
	}
}

func cleanupJob(cfg *Config) {
	slog.Info("Running cleanup job")

	cleanFile(cfg.MusicLibraryPath + "/" + "cached")

	slog.Info("Cleanup job finished.")
}
