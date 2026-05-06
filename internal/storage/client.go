package storage

import (
	"io"
	"os"
	"path/filepath"

	"github.com/scythe504/fluxstream/internal/tor"
)

type Service interface {
	SaveForLater(videoId string, reader io.Reader, meta tor.FileMetadata) (string, error)
}

type service struct {
	dataDir string
}

// New creates a new storage service, resolving the download directory with environment overrides.
func New() Service {
	// Try environment override first (for flexibility)
	dataDir := os.Getenv("DOWNLOAD_PATH")
	if dataDir == "" {
		// Default to Docker path
		dataDir = "/app/fluxstream/download"

		// Fallback for local dev environments
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			if home, err := os.UserHomeDir(); err == nil {
				dataDir = filepath.Join(home, ".local", "share", "fluxstream", "downloads")
			}
		}
	}

	// Ensure directory exists
	os.MkdirAll(dataDir, 0755)

	return &service{dataDir: dataDir}
}
