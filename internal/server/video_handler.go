package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/scythe504/fluxstream/internal"
	postgresdb "github.com/scythe504/fluxstream/internal/postgres-db"
	redisdb "github.com/scythe504/fluxstream/internal/redis-db"
	"github.com/scythe504/fluxstream/internal/tor"
)

func (s *Server) saveVideo(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("[StartVideo] Invalid Request body", err)
		http.Error(w, "Invalid json body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	var link struct {
		VideoId string `json:"video_id"`
	}

	if err = json.Unmarshal(body, &link); err != nil {
		log.Println("[StartVideo] Invalid Json Body", err)
		http.Error(w, "Failed to Parse JSON", http.StatusBadRequest)
		return
	}

	magnetLink := s.t.GetMagnetLink(link.VideoId)

	if magnetLink == nil {
		log.Println("[StartVideo] Could not get magnet link", err)
		http.Error(w, "Failed to get magnet link, please renter the magnet link to get the video", http.StatusNotFound)
		return
	}

	video := postgresdb.Video{
		Id:         link.VideoId,
		MagnetLink: *magnetLink,
		Status:     postgresdb.PROCESSING,
		FilePath:   "",
		CreatedAt:  time.Now().UTC(),
		Deleted:    false,
	}

	if err = s.db.CreateVideo(video); err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			if pgErr.Code == "23505" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(fmt.Sprintf("{ \"video_id\": \"%s\", \"message\": \"%s\" }", link.VideoId, "Video is being processed and being saved do not close the fluxstream app")))
				return
			}
		}
		log.Println("[StartVideo] Failed to generate video", err)
		http.Error(w, "Failed to get video", http.StatusInternalServerError)
		return
	}

	job := redisdb.Job{
		Id:   link.VideoId,
		Link: *magnetLink,
	}

	if err = s.rdb.PublishJob(r.Context(), job); err != nil {
		log.Println("[StartVideo] Failed to publish job", err)
		http.Error(w, "Failed to get video", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("{ \"video_id\": \"%s\", \"message\": \"%s\" }", link.VideoId, "Video is being processed and being saved do not close the fluxstream app")))
}

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

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("{ \"video_id\": \"%s\" }", videoId)))
}

// Resolve returns a video reader + metadata by checking torrent, cache, and disk.
// Order of preference:
// 1. Active torrent stream
// 2. Cached metadata or DB record + on-disk file
func (r *StreamResolver) Resolve(
	videoId string,
	getReader func(string) *torrent.Reader,
	// getVideo func(string) (postgresdb.Video, error),
	getMetadata func(string) (*tor.FileMetadata, error),
) (io.ReadSeeker, *tor.FileMetadata, error) {

	// Try torrent stream directly
	if reader := getReader(videoId); reader != nil {
		// For torrent, return basic metadata
		meta, metaErr := getMetadata(videoId)
		if metaErr != nil {
			meta = &tor.FileMetadata{
				Name:      "unknown_video",
				Path:      "",
				Length:    0,
				Extension: ".mp4",
				IsVideo:   true,
			}
		}
		return *reader, meta, nil
	}

	return nil, &tor.FileMetadata{
		Name:      "unknown_video",
		Path:      "",
		Length:    0,
		Extension: ".mp4",
		IsVideo:   true,
	}, fmt.Errorf("couldn't get reader")

	// Try cache or database
	// var video postgresdb.Video
	// if val, ok := r.cache.Load(videoId); ok {
	// 	video = val.(postgresdb.Video)
	// } else {
	// 	v, err := getVideo(videoId)
	// 	if err != nil {
	// 		return nil, nil, fmt.Errorf("failed to get video from DB: %w", err)
	// 	}
	// 	video = v
	// 	r.cache.Store(videoId, v)
	// }

	// // Try to open local file if path exists
	// if video.FilePath != "" && internal.FileExists(video.FilePath) {
	// 	f, err := os.Open(video.FilePath)
	// 	if err != nil {
	// 		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	// 	}

	// 	info, err := os.Stat(video.FilePath)
	// 	if err != nil {
	// 		f.Close()
	// 		return nil, nil, fmt.Errorf("failed to stat file: %w", err)
	// 	}

	// 	meta := &tor.FileMetadata{
	// 		Name:      filepath.Base(video.FilePath),
	// 		Path:      video.FilePath,
	// 		Length:    info.Size(),
	// 		Extension: filepath.Ext(video.FilePath),
	// 		IsVideo:   internal.IsVideoFile(filepath.Ext(video.FilePath)),
	// 	}

	// 	return f, meta, nil
	// }

	// Nothing found
	// return nil, nil, os.ErrNotExist
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

	// // Fallback: get from DB and disk
	// video, err := s.db.GetVideo(videoId)
	// if err != nil {
	// 	http.Error(w, "video not found", http.StatusNotFound)
	// 	return
	// }

	// if video.FilePath == "" || !internal.FileExists(video.FilePath) {
	// 	http.Error(w, "metadata unavailable", http.StatusNotFound)
	// 	return
	// }

	// info, err := os.Stat(video.FilePath)
	// if err != nil {
	// 	http.Error(w, "failed to read file metadata", http.StatusInternalServerError)
	// 	return
	// }

	// meta = &tor.FileMetadata{
	// 	Name:      filepath.Base(video.FilePath),
	// 	Path:      video.FilePath,
	// 	Length:    info.Size(),
	// 	Extension: filepath.Ext(video.FilePath),
	// 	IsVideo:   internal.IsVideoFile(filepath.Ext(video.FilePath)),
	// }

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

func (s *Server) streamVideo(w http.ResponseWriter, r *http.Request) {
	videoId := mux.Vars(r)["videoId"]

	// Resolve reader + metadata
	reader, meta, err := s.streamResolver.Resolve(
		videoId,
		s.t.GetReader, // Torrent getter
		// s.db.GetVideo, // DB getter
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
