package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func configureBundledFFmpegFromNativeLibraryDir(nativeLibraryDir string) error {
	nativeLibraryDir = filepath.Clean(strings.TrimSpace(nativeLibraryDir))
	if nativeLibraryDir == "" || nativeLibraryDir == "." {
		return fmt.Errorf("empty native library dir")
	}

	ffmpegPath := filepath.Join(nativeLibraryDir, "libffmpeg.so")
	ffprobePath := filepath.Join(nativeLibraryDir, "libffprobe.so")
	if err := validateBundledTool(ffmpegPath); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}
	if err := validateBundledTool(ffprobePath); err != nil {
		return fmt.Errorf("ffprobe: %w", err)
	}

	_ = os.Setenv("MUSIC_DL_FFMPEG", ffmpegPath)
	_ = os.Setenv("MUSIC_DL_FFPROBE", ffprobePath)
	prependPathDir(nativeLibraryDir)
	return nil
}

func validateBundledTool(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%q is a directory", path)
	}
	_ = os.Chmod(path, 0755)
	return nil
}

func prependPathDir(dir string) {
	current := os.Getenv("PATH")
	for _, entry := range filepath.SplitList(current) {
		if filepath.Clean(entry) == dir {
			return
		}
	}
	if current == "" {
		_ = os.Setenv("PATH", dir)
		return
	}
	_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+current)
}
