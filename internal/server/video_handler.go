package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/gorilla/mux"
	"github.com/scythe504/fluxstream/internal"
	"github.com/scythe504/fluxstream/internal/database"
	"github.com/scythe504/fluxstream/internal/tor"
)

func (s *Server) listVideos(w http.ResponseWriter, r *http.Request) {
	videos, err := s.db.GetAllVideos()
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to fetch videos: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(videos); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}

func (s *Server) createVideo(w http.ResponseWriter, r *http.Request) {
	// 1. Get magnet link from request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("[StartVideo] Invalid Request body", err)
		http.Error(w, "Invalid json body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	var link struct {
		MagnetLink string `json:"magnet_link"`
	}

	if err = json.Unmarshal(body, &link); err != nil {
		log.Println("[StartVideo] Invalid Json Body", err)
		http.Error(w, "Failed to Parse JSON", http.StatusBadRequest)
		return
	}

	videoId := internal.RandomId()

	if err = s.t.AddMagnet(videoId, link.MagnetLink); err != nil {
		log.Println("[StartVideo] failed to get the magnet link")
		http.Error(w, "failed to get video", http.StatusBadRequest)
		return
	}

	filePath, err := s.t.GetMetadata(videoId)
	video := database.Video{
		Id:         videoId,
		MagnetLink: link.MagnetLink,
		FilePath:   filepath.Join(os.Getenv("DOWNLOAD_PATH"), filePath.Name),
		CreatedAt:  time.Now().UnixMilli(),
	}
	// Insert Video row into SQLite
	if err = s.db.CreateVideo(video); err != nil {
		log.Println("[StartVideo] failed to persist video", err)
		http.Error(w, "failed to persist video", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("{ \"video_id\": \"%s\" }", videoId)))
}

// Resolve returns a video reader + metadata by checking torrent, cache, and disk.
// Order of preference:
// 1. Active torrent stream
// 2. Cached metadata or DB record + on-disk file
func ResolveStream(
	videoId string,
	getReader func(string) *torrent.Reader,
	getVideo func(string) (database.Video, error),
	addMagnet func(string, string) error,
	getMetadata func(string) (*tor.FileMetadata, error),
) (io.ReadSeeker, *tor.FileMetadata, error) {

	// Try torrent stream directly
	if reader := getReader(videoId); reader != nil {
		meta, err := getMetadata(videoId)
		if err != nil {
			meta = &tor.FileMetadata{
				Name:      "unknown_video",
				Extension: ".mp4",
				IsVideo:   true,
			}
		}
		return *reader, meta, nil
	}

	// Not in memory, fetch from DB and re-add magnet
	video, err := getVideo(videoId)
	if err != nil {
		return nil, nil, fmt.Errorf("video not found: %w", err)
	}

	if err := addMagnet(videoId, video.MagnetLink); err != nil {
		return nil, nil, fmt.Errorf("failed to resume torrent: %w", err)
	}

	reader := getReader(videoId)
	if reader == nil {
		return nil, nil, fmt.Errorf("failed to get reader after resume")
	}

	meta, err := getMetadata(videoId)
	if err != nil {
		meta = &tor.FileMetadata{
			Name:      "unknown_video",
			Extension: ".mp4",
			IsVideo:   true,
		}
	}

	return *reader, meta, nil
}

func (s *Server) getVideoMetadata(w http.ResponseWriter, r *http.Request) {
	videoId := mux.Vars(r)["videoId"]

	// Try torrent first (if active)
	meta, err := s.t.GetMetadata(videoId)
	if err == nil && meta != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(meta)
		return
	}

	// Fallback: get from DB and disk
	video, err := s.db.GetVideo(videoId)
	if err != nil {
		http.Error(w, "video not found", http.StatusNotFound)
		return
	}

	if video.FilePath == "" || !internal.FileExists(video.FilePath) {
		http.Error(w, "metadata unavailable", http.StatusNotFound)
		return
	}

	info, err := os.Stat(video.FilePath)
	if err != nil {
		http.Error(w, "failed to read file metadata", http.StatusInternalServerError)
		return
	}

	meta = &tor.FileMetadata{
		Name:      filepath.Base(video.FilePath),
		Path:      video.FilePath,
		Length:    info.Size(),
		Extension: filepath.Ext(video.FilePath),
		IsVideo:   internal.IsVideoFile(filepath.Ext(video.FilePath)),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

func (s *Server) streamVideo(w http.ResponseWriter, r *http.Request) {
	videoId := mux.Vars(r)["videoId"]

	// Resolve reader + metadata
	reader, meta, err := ResolveStream(
		videoId,
		s.t.GetReader, // Torrent getter
		s.db.GetVideo, // DB getter
		s.t.AddMagnet,
		s.t.GetMetadata,
	)
	if err != nil {
		http.Error(w, "video not found", http.StatusNotFound)
		return
	}
	defer func() {
		if c, ok := reader.(io.Closer); ok {
			c.Close()
		}
	}()

	// Set response headers
	contentType := mime.TypeByExtension(meta.Extension)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")

	// Stream with actual filename
	http.ServeContent(w, r, meta.Name, time.Now(), reader)
}