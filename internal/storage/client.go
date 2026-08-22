package storage

import (
	"io"
	"os"

	"github.com/scythe504/fluxstream/internal/tor"
	"github.com/scythe504/fluxstream/internal/utils"
)

type Service interface {
	SaveForLater(videoId string, reader io.Reader, meta tor.FileMetadata) (string, error)
}

type service struct {
	dataDir string
}

// New creates a new storage service, resolving the download directory with environment overrides.
func New() Service {
	dataDir := utils.GetDownloadDir()
	_ = os.MkdirAll(dataDir, 0755)

	return &service{dataDir: dataDir}
}
