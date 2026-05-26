package tor

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/scythe504/fluxstream/internal/utils"
)

type TorrentManager struct {
	cl       *torrent.Client
	torrents map[string]*torrent.Torrent
	mu       sync.RWMutex
}

type FileMetadata struct {
	Name      string `json:"name"`      // e.g. "movie.mkv"
	Path      string `json:"path"`      // Full path within torrent
	Length    int64  `json:"length"`    // File size in bytes
	Extension string `json:"extension"` // e.g. ".mp4"
	IsVideo   bool   `json:"is_video"`  // Whether it's a recognized video format
}

func New(port int) TorrentManager {
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

	return TorrentManager{
		cl:       client,
		torrents: make(map[string]*torrent.Torrent),
		mu:       sync.RWMutex{},
	}
}

func (tr *TorrentManager) AddMagnet(id, magnetLink string) error {
	tr.mu.Lock()
	t, err := tr.cl.AddMagnet(magnetLink)
	if err != nil {
		tr.mu.Unlock()
		return fmt.Errorf("failed to add magnet: %w", err)
	}

	// Wait for torrent metadata (with timeout)
	select {
	case <-t.GotInfo():
	case <-time.After(10 * time.Second):
		t.Drop()
		tr.mu.Unlock()
		return fmt.Errorf("timeout waiting for metadata for id: %s", id)
	}

	files := t.Files()
	if len(files) == 0 {
		t.Drop()
		tr.mu.Unlock()
		return fmt.Errorf("no files found in torrent for id: %s", id)
	}

	// Check if at least one valid video file exists
	hasVideo := false
	for _, f := range files {
		if utils.IsVideoFile(filepath.Ext(f.DisplayPath())) {
			hasVideo = true
			break
		}
	}

	if !hasVideo {
		t.Drop() // prevent keeping useless torrents
		tr.mu.Unlock()
		return fmt.Errorf("no valid video files found for id: %s", id)
	}

	// Save torrent handle
	tr.torrents[id] = t
	tr.mu.Unlock()
	return nil
}
func (tr *TorrentManager) GetReader(id string) *torrent.Reader {
	// Ensure torrent exists
	tr.mu.RLock()
	t, ok := tr.torrents[id]
	if !ok || t == nil {
		tr.mu.RUnlock()
		return nil
	}

	// Wait for metadata
	select {
	case <-t.GotInfo():
	case <-time.After(15 * time.Second):
		log.Printf("[GetReader] timeout waiting for metadata: %s", id)
		tr.mu.RUnlock()
		return nil
	}

	// Get the main video file
	mainFile, err := tr.getMainVideoFile(id)
	if err != nil {
		log.Printf("[GetReader] failed to get main video file: %v", err)
		tr.mu.RUnlock()
		return nil
	}

	// Find the file that matches the path in metadata
	reader := mainFile.NewReader()
	if reader == nil {
		log.Printf("[GetReader] failed to create reader for file: %s", mainFile.DisplayPath())
		tr.mu.RUnlock()
		return nil
	}
	tr.mu.RUnlock()
	return &reader
}
func (tr *TorrentManager) GetMagnetLink(videoId string) *string {
	tr.mu.RLock()
	t, ok := tr.torrents[videoId]
	if !ok || t == nil {
		log.Printf("[GetMagnetLink] torrent not found for id: %s", videoId)
		tr.mu.RUnlock()
		return nil
	}

	metainfo := t.Metainfo()

	magnetV2, err := metainfo.MagnetV2()
	if err != nil {
		log.Printf("[GetMagnetLink] failed to get magnet V2: %v", err)
		tr.mu.RUnlock()
		return nil
	}

	magnetURI := magnetV2.String()
	tr.mu.RUnlock()
	return &magnetURI
}

func (tr *TorrentManager) CleanupTorrent(videoId string) error {
	tr.mu.Lock()
	t, ok := tr.torrents[videoId]
	if !ok || t == nil {
		log.Printf("[CleanupTorrent] no active torrent found for id: %s", videoId)
		tr.mu.Unlock()
		return nil
	}
	delete(tr.torrents, videoId)
	tr.mu.Unlock()

	t.Drop()
	log.Printf("[CleanupTorrent] cleaned up torrent for id: %s", videoId)
	return nil
}

// GetMainVideoFile returns the largest valid video file in the torrent.
func (tr *TorrentManager) getMainVideoFile(videoId string) (*torrent.File, error) {
	t, ok := tr.torrents[videoId]
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
		if !utils.IsVideoFile(ext) {
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
func (tr *TorrentManager) GetMetadata(videoId string) (*FileMetadata, error) {
	tr.mu.RLock()
	mainFile, err := tr.getMainVideoFile(videoId)
	if err != nil {
		tr.mu.RUnlock()
		return nil, err
	}

	path := mainFile.DisplayPath()
	ext := strings.ToLower(filepath.Ext(path))

	meta := &FileMetadata{
		Name:      filepath.Base(path),
		Path:      path,
		Length:    mainFile.Length(),
		Extension: ext,
		IsVideo:   utils.IsVideoFile(ext),
	}

	tr.mu.RUnlock()
	return meta, nil
}

type TorrentStats struct {
	BytesCompleted int64   `json:"bytes_completed"`
	BytesMissing   int64   `json:"bytes_missing"`
	TotalBytes     int64   `json:"total_bytes"`
	Progress       float64 `json:"progress"`
	ActivePeers    int     `json:"active_peers"`
	TotalPeers     int     `json:"total_peers"`
	DownloadSpeed  int64   `json:"download_speed"`
	UploadSpeed    int64   `json:"upload_speed"`
}

func (tr *TorrentManager) GetStats(videoId string, prevDown, prevUp int64) (*TorrentStats, int64, int64, error) {
	tr.mu.RLock()
	t, ok := tr.torrents[videoId]
	tr.mu.RUnlock()
	if !ok || t == nil {
		return nil, 0, 0, fmt.Errorf("torrent not found: %s", videoId)
	}

	stats := t.Stats()

	missing := t.BytesMissing()
	completed := t.BytesCompleted()
	total := completed + missing

	currentDown := stats.BytesRead.Int64()
	currentUp := stats.BytesWritten.Int64()

	var progress float64
	if total > 0 {
		progress = float64(completed) / float64(total)
	}

	return &TorrentStats{
		BytesCompleted: completed,
		BytesMissing:   missing,
		TotalBytes:     total,
		Progress:       progress,
		ActivePeers:    stats.ActivePeers,
		TotalPeers:     stats.TotalPeers,
		DownloadSpeed:  currentDown - prevDown,
		UploadSpeed:    currentUp - prevUp,
	}, currentDown, currentUp, nil
}
