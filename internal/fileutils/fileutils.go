package fileutils

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// fileinfo represents metadata about a file
type FileInfo struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	Checksum     string    `json:"checksum"`
	RelativePath string    `json:"relative_path"`
}

// scandirectory walks a directory and returns a list of files.
func ScanDirectory(rootDir string) ([]string, error) {
	var files []string
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

// scandirectorywithinfo walks a directory and returns detailed file information
func ScanDirectoryWithInfo(rootDir string) ([]FileInfo, error) {
	var files []FileInfo

	// ensure root directory exists
	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", rootDir)
	}

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// skip directories
		if info.IsDir() {
			return nil
		}

		// skip hidden files and system files
		if filepath.Base(path)[0] == '.' {
			return nil
		}

		// get relative path from root
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		// calculate file checksum
		checksum, err := calculateFileChecksum(path)
		if err != nil {
			return fmt.Errorf("failed to calculate checksum for %s: %w", path, err)
		}

		fileInfo := FileInfo{
			Path:         path,
			Size:         info.Size(),
			ModTime:      info.ModTime(),
			Checksum:     checksum,
			RelativePath: relPath,
		}

		files = append(files, fileInfo)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error scanning directory: %w", err)
	}

	return files, nil
}

// calculatefilechecksum computes the sha256 checksum of a file
func calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// fileexists checks if a file exists
func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

// createdirifnotexists creates a directory if it does not exist
func CreateDirIfNotExists(dirPath string) error {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return os.MkdirAll(dirPath, 0755)
	}
	return nil
}
