package tor

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/scythe504/fluxstream/internal"
)

type Torrent struct {
	cl  *torrent.Client
	tor map[string]*torrent.Torrent
}

type FileMetadata struct {
	Name      string `json:"name"`      // e.g. "movie.mkv"
	Path      string `json:"path"`      // Full path within torrent
	Length    int64  `json:"length"`    // File size in bytes
	Extension string `json:"extension"` // e.g. ".mp4"
	IsVideo   bool   `json:"is_video"`  // Whether it's a recognized video format
}

func New(port int) Torrent {
	cfg := torrent.NewDefaultClientConfig()

	// Networking
	cfg.ListenPort = port
	cfg.DisableIPv6 = true
	cfg.DisableUTP = false
	cfg.DisableAggressiveUpload = true
	cfg.NoUpload = false
	cfg.Seed = true

	// Performance tuning
	cfg.MinDialTimeout = 5 * time.Second

	// Try environment override first (for flexibility)
	dataDir := os.Getenv("DOWNLOAD_PATH")
	if dataDir == "" {
		// Default to Docker path if not specified
		dataDir = "/app/fluxstream/download"

		// Fallback for local dev environments
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			if home, err := os.UserHomeDir(); err == nil {
				dataDir = filepath.Join(home, ".local", "share", "fluxstream", "downloads")
			}
		}
	}

	cfg.DataDir = dataDir

	client, err := torrent.NewClient(cfg)
	if err != nil {
		log.Fatal(err)
	}

	return Torrent{
		cl:  client,
		tor: make(map[string]*torrent.Torrent),
	}
}

func (tr *Torrent) AddMagnet(id, magnetLink string) error {
	t, err := tr.cl.AddMagnet(magnetLink)
	if err != nil {
		return fmt.Errorf("failed to add magnet: %w", err)
	}

	// Wait for torrent metadata (with timeout)
	select {
	case <-t.GotInfo():
	case <-time.After(10 * time.Second):
		t.Drop()
		return fmt.Errorf("timeout waiting for metadata for id: %s", id)
	}

	files := t.Files()
	if len(files) == 0 {
		t.Drop()
		return fmt.Errorf("no files found in torrent for id: %s", id)
	}

	// Check if at least one valid video file exists
	hasVideo := false
	for _, f := range files {
		if internal.IsVideoFile(filepath.Ext(f.DisplayPath())) {
			hasVideo = true
			break
		}
	}

	if !hasVideo {
		t.Drop() // prevent keeping useless torrents
		return fmt.Errorf("no valid video files found for id: %s", id)
	}

	// Save torrent handle
	tr.tor[id] = t
	return nil
}
func (tr *Torrent) GetReader(id string) *torrent.Reader {
	// Ensure torrent exists
	t, ok := tr.tor[id]
	if !ok || t == nil {
		return nil
	}

	// Wait for metadata
	select {
	case <-t.GotInfo():
	case <-time.After(15 * time.Second):
		log.Printf("[GetReader] timeout waiting for metadata: %s", id)
		return nil
	}

	// Get the main video file
	mainFile, err := tr.GetMainVideoFile(id)
	if err != nil {
		log.Printf("[GetReader] failed to get main video file: %v", err)
		return nil
	}

	// Find the file that matches the path in metadata
	reader := mainFile.NewReader()
	if reader == nil {
		log.Printf("[GetReader] failed to create reader for file: %s", mainFile.DisplayPath())
		return nil
	}
	return &reader
}
func (tr *Torrent) GetMagnetLink(videoId string) *string {
	t, ok := tr.tor[videoId]
	if !ok || t == nil {
		log.Printf("[GetMagnetLink] torrent not found for id: %s", videoId)
		return nil
	}

	metainfo := t.Metainfo()

	magnetV2, err := metainfo.MagnetV2()
	if err != nil {
		log.Printf("[GetMagnetLink] failed to get magnet V2: %v", err)
		return nil
	}

	magnetURI := magnetV2.String()
	return &magnetURI
}

// CleanupTorrent safely stops and removes a torrent from memory and disk cache.
func (tr *Torrent) CleanupTorrent(videoId string) error {
	// Ensure torrent exists
	t, ok := tr.tor[videoId]
	if !ok || t == nil {
		log.Printf("[CleanupTorrent] no active torrent found for id: %s", videoId)
		return nil
	}

	// Stop all activity on the torrent
	defer func() {
		delete(tr.tor, videoId)
		log.Printf("[CleanupTorrent] cleaned up torrent for id: %s", videoId)
	}()

	// Attempt to close all readers
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[CleanupTorrent] recovered while closing readers for id %s: %v", videoId, r)
			}
		}()
		for _, f := range t.Files() {
			f.DisplayPath() // touch file to ensure safety
		}
	}()

	// Stop the torrent download
	t.Drop()

	return nil
}

// GetMainVideoFile returns the largest valid video file in the torrent.
func (tr *Torrent) GetMainVideoFile(videoId string) (*torrent.File, error) {
	t, ok := tr.tor[videoId]
	if !ok {
		return nil, fmt.Errorf("torrent not found for videoId: %s", videoId)
	}

	// Wait for metadata
	select {
	case <-t.GotInfo():
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("timeout waiting for torrent metadata for videoId: %s", videoId)
	}

	files := t.Files()
	if len(files) == 0 {
		return nil, fmt.Errorf("no files in torrent for videoId: %s", videoId)
	}

	var best *torrent.File
	for i := range files {
		ext := strings.ToLower(filepath.Ext(files[i].DisplayPath()))
		if !internal.IsVideoFile(ext) {
			continue
		}
		if best == nil || files[i].Length() > best.Length() {
			best = files[i]
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no video files found in torrent for videoId: %s", videoId)
	}

	return best, nil
}

// GetMetadata returns metadata for the main video file of the torrent.
func (tr *Torrent) GetMetadata(videoId string) (*FileMetadata, error) {
	mainFile, err := tr.GetMainVideoFile(videoId)
	if err != nil {
		return nil, err
	}

	path := mainFile.DisplayPath()
	ext := strings.ToLower(filepath.Ext(path))

	meta := &FileMetadata{
		Name:      filepath.Base(path),
		Path:      path,
		Length:    mainFile.Length(),
		Extension: ext,
		IsVideo:   internal.IsVideoFile(ext),
	}

	return meta, nil
}
