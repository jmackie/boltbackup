package main

import (
	"os"
	"path/filepath"
	"runtime"
)

// Walk a file tree and grow collection with only files.
func justFiles(collection *[]string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err // propagate
		}
		if info.IsDir() {
			return nil // ignore
		}
		*collection = append(*collection, path)
		return nil
	}
}

// Get "~" in a cross-platform way.
func homeDir() (home string) {
	switch runtime.GOOS {
	case "windows":
		home = os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
	default:
		home = os.Getenv("HOME")
	}
	return
}
