package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/gorilla/mux"
	"github.com/scythe504/fluxstream/internal/database"
	"github.com/scythe504/fluxstream/internal/tor"
	"github.com/scythe504/fluxstream/internal/utils"
)

func (s *Server) listVideos(w http.ResponseWriter, r *http.Request) {
	videos, err := s.db.GetAllVideos()
	if err != nil {
		utils.LogHandlerError(r, "listVideos", err, nil)
		utils.WriteError(w, http.StatusInternalServerError, "failed to fetch videos")
		return
	}

	// Trigger background loading of any inactive torrents from the database
	for _, video := range videos {
		go func(v database.Video) {
			if reader := s.t.GetReader(v.Id); reader == nil {
				log.Printf("[listVideos] Auto-resuming torrent in background: %s", v.Id)
				if _, err := s.t.AddMagnet(v.MagnetLink); err != nil {
					log.Printf("[listVideos] Failed to auto-resume torrent %s in background: %v", v.Id, err)
				}
			}
		}(video)
	}

	utils.WriteJSON(w, http.StatusOK, videos)
}

func (s *Server) createVideo(w http.ResponseWriter, r *http.Request) {
	// Get magnet link from request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		utils.LogHandlerError(r, "createVideo", err, nil)
		utils.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	defer r.Body.Close()
	var link struct {
		MagnetLink string `json:"magnet_link"`
	}

	if err = json.Unmarshal(body, &link); err != nil {
		utils.LogHandlerError(r, "createVideo", err, nil)
		utils.WriteError(w, http.StatusBadRequest, "failed to parse JSON body")
		return
	}

	infoHash, err := s.t.AddMagnet(link.MagnetLink)
	if err != nil {
		utils.LogHandlerError(r, "createVideo", err, map[string]any{"magnetLink": link.MagnetLink})
		utils.WriteError(w, http.StatusBadRequest, "failed to add magnet link")
		return
	}

	filePath, err := s.t.GetMetadata(infoHash)
	if err != nil {
		utils.LogHandlerError(r, "createVideo", err, map[string]any{"videoId": infoHash})
		utils.WriteError(w, http.StatusInternalServerError, "failed to retrieve torrent metadata")
		return
	}

	video := database.Video{
		Id:         infoHash,
		MagnetLink: link.MagnetLink,
		FilePath:   filepath.Join(s.t.GetDataDir(), filePath.Path),
		CreatedAt:  time.Now().UnixMilli(),
	}
	if err = s.db.CreateVideo(video); err != nil {
		// already exists, not an error — just continue
		utils.LogHandlerError(r, "createVideo", err, map[string]any{"videoId": infoHash})
		utils.WriteError(w, http.StatusInternalServerError, "failed to persist video metadata")
		return
	}
	utils.WriteJSON(w, http.StatusOK, map[string]string{"video_id": infoHash})
}

func (s *Server) getDownloadDir() string {
	if s.t.GetDataDir() != "" {
		return s.t.GetDataDir()
	}
	return utils.GetDownloadDir()
}

func (s *Server) resolveVideoFilePath(videoId string, video database.Video) (string, error) {
	dataDir := s.getDownloadDir()

	// 1. Check if active torrent metadata gives the accurate path
	if meta, err := s.t.GetMetadata(videoId); err == nil && meta != nil && meta.Path != "" {
		expectedPath := filepath.Join(dataDir, meta.Path)
		if utils.FileExists(expectedPath) {
			return expectedPath, nil
		}
		if utils.FileExists(expectedPath + ".part") {
			return expectedPath + ".part", nil
		}
	}

	// 2. Check if video.FilePath exists directly or with .part
	if video.FilePath != "" {
		if utils.FileExists(video.FilePath) {
			return video.FilePath, nil
		}
		if utils.FileExists(video.FilePath + ".part") {
			return video.FilePath + ".part", nil
		}

		// Check relative to current data directory
		relPath := filepath.Join(dataDir, video.FilePath)
		if utils.FileExists(relPath) {
			return relPath, nil
		}
		if utils.FileExists(relPath + ".part") {
			return relPath, nil
		}

		// Check basename in current data directory
		baseName := filepath.Base(video.FilePath)
		basePath := filepath.Join(dataDir, baseName)
		if utils.FileExists(basePath) {
			return basePath, nil
		}
		if utils.FileExists(basePath + ".part") {
			return basePath + ".part", nil
		}

		// Search subdirectories within dataDir
		var foundPath string
		targetBase := baseName
		targetPart := baseName + ".part"

		_ = filepath.WalkDir(dataDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			name := d.Name()
			if name == targetBase || name == targetPart {
				foundPath = path
				return filepath.SkipAll
			}
			return nil
		})

		if foundPath != "" {
			return foundPath, nil
		}
	}

	return "", fmt.Errorf("video file not found on disk: %s", video.FilePath)
}

// Resolve returns a video reader + metadata by checking torrent, cache, and disk.
// Order of preference:
// - Active torrent stream
// - Cached metadata or DB record + on-disk file
func ResolveStream(
	videoId string,
	getReader func(string) *torrent.Reader,
	getVideo func(string) (database.Video, error),
	addMagnet func(string) (string, error),
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

	if _, err := addMagnet(video.MagnetLink); err != nil {
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
		utils.WriteJSON(w, http.StatusOK, meta)
		return
	}

	// Fallback: get from DB and disk
	video, err := s.db.GetVideo(videoId)
	if err != nil {
		utils.LogHandlerError(r, "getVideoMetadata", err, map[string]any{"videoId": videoId})
		utils.WriteError(w, http.StatusNotFound, "video not found")
		return
	}

	actualPath, err := s.resolveVideoFilePath(videoId, video)
	if err != nil {
		utils.LogHandlerError(r, "getVideoMetadata", err, map[string]any{"videoId": videoId, "filePath": video.FilePath})
		utils.WriteError(w, http.StatusNotFound, "metadata unavailable")
		return
	}

	info, err := os.Stat(actualPath)
	if err != nil {
		utils.LogHandlerError(r, "getVideoMetadata", err, map[string]any{"videoId": videoId, "filePath": actualPath})
		utils.WriteError(w, http.StatusInternalServerError, "failed to read file metadata")
		return
	}

	cleanPath := strings.TrimSuffix(actualPath, ".part")
	meta = &tor.FileMetadata{
		Name:      filepath.Base(cleanPath),
		Path:      cleanPath,
		Length:    info.Size(),
		Extension: filepath.Ext(cleanPath),
		IsVideo:   utils.IsVideoFile(filepath.Ext(cleanPath)),
	}

	utils.WriteJSON(w, http.StatusOK, meta)
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
		utils.LogHandlerError(r, "streamVideo", err, map[string]any{"videoId": videoId})
		utils.WriteError(w, http.StatusNotFound, "video not found")
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

func (s *Server) serveSubtitles(w http.ResponseWriter, r *http.Request) {
	videoId := mux.Vars(r)["videoId"]

	video, err := s.db.GetVideo(videoId)
	if err != nil {
		utils.LogHandlerError(r, "serveSubtitles", err, map[string]any{"videoId": videoId})
		utils.WriteError(w, http.StatusNotFound, "no file found in database")
		return
	}

	inputPath, err := s.resolveVideoFilePath(videoId, video)
	if err != nil {
		utils.LogHandlerError(r, "serveSubtitles", err, map[string]any{"videoId": videoId, "filePath": video.FilePath})
		utils.WriteError(w, http.StatusNotFound, "video file not found on disk")
		return
	}

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	cmd := exec.CommandContext(r.Context(), "ffmpeg",
		"-i", inputPath, "-map", "0:s:0",
		"-c:s", "webvtt", "-f", "webvtt", "pipe:1", "-y",
	)
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	w.Header().Set("Content-Type", "text/vtt")

	err = cmd.Run()
	if err != nil {
		utils.LogHandlerError(r, "serveSubtitles", err, map[string]any{
			"videoId":      videoId,
			"inputPath":    inputPath,
			"ffmpegStderr": stderrBuf.String(),
		})
		// Fallback to a valid empty WebVTT file if extraction fails (e.g., no subtitles in MP4/AVI)
		w.Write([]byte("WEBVTT\n"))
		return
	}

	w.Write(stdoutBuf.Bytes())
}
