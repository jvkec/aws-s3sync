package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jvkec/aws-s3sync/internal/fileutils"
)

// manifest represents the sync state of files
type Manifest struct {
	LastSync time.Time                     `json:"last_sync"`
	Files    map[string]fileutils.FileInfo `json:"files"`
	Bucket   string                        `json:"bucket"`
	Prefix   string                        `json:"prefix"`
}

// manifestmanager handles manifest operations
type ManifestManager struct {
	manifestPath string
}

// newmanifestmanager creates a new manifest manager
func NewManifestManager(localPath string) *ManifestManager {
	manifestPath := filepath.Join(localPath, ".s3sync", "manifest.json")
	return &ManifestManager{
		manifestPath: manifestPath,
	}
}

// loadmanifest loads an existing manifest from disk
func (m *ManifestManager) LoadManifest() (*Manifest, error) {
	if !fileutils.FileExists(m.manifestPath) {
		// return empty manifest if none exists
		return &Manifest{
			Files: make(map[string]fileutils.FileInfo),
		}, nil
	}

	data, err := os.ReadFile(m.manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest file: %w", err)
	}

	if manifest.Files == nil {
		manifest.Files = make(map[string]fileutils.FileInfo)
	}

	return &manifest, nil
}

// savemanifest saves the manifest to disk
func (m *ManifestManager) SaveManifest(manifest *Manifest) error {
	// ensure manifest directory exists
	manifestDir := filepath.Dir(m.manifestPath)
	if err := fileutils.CreateDirIfNotExists(manifestDir); err != nil {
		return fmt.Errorf("failed to create manifest directory: %w", err)
	}

	manifest.LastSync = time.Now()

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := os.WriteFile(m.manifestPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest file: %w", err)
	}

	return nil
}

// buildlocalmanifest creates a manifest from current local directory state
func (m *ManifestManager) BuildLocalManifest(localPath string) (*Manifest, error) {
	files, err := fileutils.ScanDirectoryWithInfo(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	manifest := &Manifest{
		Files:    make(map[string]fileutils.FileInfo),
		LastSync: time.Now(),
	}

	for _, file := range files {
		manifest.Files[file.RelativePath] = file
	}

	return manifest, nil
}

// syncop represents a sync operation type
type SyncOp string

const (
	SyncOpUpload   SyncOp = "upload"
	SyncOpDownload SyncOp = "download"
	SyncOpDelete   SyncOp = "delete"
	SyncOpSkip     SyncOp = "skip"
)

// syncaction represents an action to be taken during sync
type SyncAction struct {
	Operation    SyncOp             `json:"operation"`
	File         fileutils.FileInfo `json:"file"`
	RelativePath string             `json:"relative_path"`
	Reason       string             `json:"reason"`
}

// computesyncactions compares local and remote manifests to determine sync actions
func ComputeSyncActions(localManifest, remoteManifest, lastKnownManifest *Manifest) []SyncAction {
	actions := make([]SyncAction, 0)

	// create maps for efficient lookup
	localFiles := localManifest.Files
	remoteFiles := remoteManifest.Files
	lastKnownFiles := lastKnownManifest.Files

	// check all local files
	for relativePath, localFile := range localFiles {
		remoteFile, remoteExists := remoteFiles[relativePath]
		lastKnownFile, wasKnown := lastKnownFiles[relativePath]

		if !remoteExists {
			// file exists locally but not remotely
			if !wasKnown {
				// new local file - upload
				actions = append(actions, SyncAction{
					Operation:    SyncOpUpload,
					File:         localFile,
					RelativePath: relativePath,
					Reason:       "new local file",
				})
			} else {
				// file was deleted remotely - need to decide conflict resolution
				actions = append(actions, SyncAction{
					Operation:    SyncOpUpload,
					File:         localFile,
					RelativePath: relativePath,
					Reason:       "deleted remotely, exists locally",
				})
			}
		} else {
			// file exists both locally and remotely
			if localFile.Checksum != remoteFile.Checksum {
				// files differ - check which one is newer
				if wasKnown {
					localChanged := localFile.Checksum != lastKnownFile.Checksum
					remoteChanged := remoteFile.Checksum != lastKnownFile.Checksum

					if localChanged && !remoteChanged {
						// only local changed - upload
						actions = append(actions, SyncAction{
							Operation:    SyncOpUpload,
							File:         localFile,
							RelativePath: relativePath,
							Reason:       "local file modified",
						})
					} else if !localChanged && remoteChanged {
						// only remote changed - download
						actions = append(actions, SyncAction{
							Operation:    SyncOpDownload,
							File:         remoteFile,
							RelativePath: relativePath,
							Reason:       "remote file modified",
						})
					} else {
						// both changed - use modification time to resolve
						if localFile.ModTime.After(remoteFile.ModTime) {
							actions = append(actions, SyncAction{
								Operation:    SyncOpUpload,
								File:         localFile,
								RelativePath: relativePath,
								Reason:       "local file newer (conflict resolution)",
							})
						} else {
							actions = append(actions, SyncAction{
								Operation:    SyncOpDownload,
								File:         remoteFile,
								RelativePath: relativePath,
								Reason:       "remote file newer (conflict resolution)",
							})
						}
					}
				} else {
					// file not in last known state - use modification time
					if localFile.ModTime.After(remoteFile.ModTime) {
						actions = append(actions, SyncAction{
							Operation:    SyncOpUpload,
							File:         localFile,
							RelativePath: relativePath,
							Reason:       "local file newer",
						})
					} else {
						actions = append(actions, SyncAction{
							Operation:    SyncOpDownload,
							File:         remoteFile,
							RelativePath: relativePath,
							Reason:       "remote file newer",
						})
					}
				}
			} else {
				// files are identical - skip
				actions = append(actions, SyncAction{
					Operation:    SyncOpSkip,
					File:         localFile,
					RelativePath: relativePath,
					Reason:       "files identical",
				})
			}
		}
	}

	// check for files that exist remotely but not locally
	for relativePath, remoteFile := range remoteFiles {
		if _, localExists := localFiles[relativePath]; !localExists {
			if _, wasKnown := lastKnownFiles[relativePath]; wasKnown {
				// file was deleted locally - skip download (respect local deletion)
				actions = append(actions, SyncAction{
					Operation:    SyncOpSkip,
					File:         remoteFile,
					RelativePath: relativePath,
					Reason:       "deleted locally",
				})
			} else {
				// new remote file - download
				actions = append(actions, SyncAction{
					Operation:    SyncOpDownload,
					File:         remoteFile,
					RelativePath: relativePath,
					Reason:       "new remote file",
				})
			}
		}
	}

	return actions
}
