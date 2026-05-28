package storage

import (
	"fmt"
	"github.com/scythe504/fluxstream/internal/tor"
	"io"
	"os"
	"path/filepath"
)

// SaveForLater saves a torrent's video file using metadata to determine filename & extension.
func (s *service) SaveForLater(videoId string, reader io.Reader, meta tor.FileMetadata) (string, error) {
	if !meta.IsVideo {
		return "", fmt.Errorf("file %s is not a recognized video type", meta.Name)
	}

	fileName := meta.Name
	targetPath := filepath.Join(s.dataDir, fileName)

	file, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file %s: %w", targetPath, err)
	}
	defer file.Close()

	buf := make([]byte, 1024*1024) // 1MB buffer
	var totalWritten int64
	var lastLogged int64

	var totalSize int64
	if meta.Length > 0 {
		totalSize = meta.Length
	}

	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			written, writeErr := file.Write(buf[:n])
			if writeErr != nil {
				return "", fmt.Errorf("failed to write file data for %s: %w", videoId, writeErr)
			}
			totalWritten += int64(written)

			// log every 5% or every 50MB if size unknown
			if totalSize > 0 {
				progress := (totalWritten * 100) / totalSize
				if progress >= lastLogged+5 {
					fmt.Printf("[%s] Progress: %d%%\n", videoId, progress)
					lastLogged = progress
				}
			} else if totalWritten-lastLogged >= 50*1024*1024 {
				fmt.Printf("[%s] Written %d MB\n", videoId, totalWritten/1024/1024)
				lastLogged = totalWritten
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("failed to read data for %s: %w", videoId, readErr)
		}
	}

	fmt.Printf("[%s] Completed saving %s (%.2f MB)\n", videoId, meta.Name, float64(totalWritten)/1024/1024)
	return targetPath, nil
}
