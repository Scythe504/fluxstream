package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

	videoId := utils.RandomId()

	if err = s.t.AddMagnet(videoId, link.MagnetLink); err != nil {
		utils.LogHandlerError(r, "createVideo", err, map[string]any{"magnetLink": link.MagnetLink})
		utils.WriteError(w, http.StatusBadRequest, "failed to add magnet link")
		return
	}

	filePath, err := s.t.GetMetadata(videoId)
	if err != nil {
		utils.LogHandlerError(r, "createVideo", err, map[string]any{"videoId": videoId})
		utils.WriteError(w, http.StatusInternalServerError, "failed to retrieve torrent metadata")
		return
	}

	video := database.Video{
		Id:         videoId,
		MagnetLink: link.MagnetLink,
		FilePath:   filepath.Join(os.Getenv("DOWNLOAD_PATH"), filePath.Name),
		CreatedAt:  time.Now().UnixMilli(),
	}
	// Insert Video row into SQLite
	if err = s.db.CreateVideo(video); err != nil {
		utils.LogHandlerError(r, "createVideo", err, map[string]any{"videoId": videoId, "video": video})
		utils.WriteError(w, http.StatusInternalServerError, "failed to persist video metadata")
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]string{"video_id": videoId})
}

// Resolve returns a video reader + metadata by checking torrent, cache, and disk.
// Order of preference:
// - Active torrent stream
// - Cached metadata or DB record + on-disk file
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

	if video.FilePath == "" || !utils.FileExists(video.FilePath) {
		err := fmt.Errorf("video file does not exist on disk: %s", video.FilePath)
		utils.LogHandlerError(r, "getVideoMetadata", err, map[string]any{"videoId": videoId})
		utils.WriteError(w, http.StatusNotFound, "metadata unavailable")
		return
	}

	info, err := os.Stat(video.FilePath)
	if err != nil {
		utils.LogHandlerError(r, "getVideoMetadata", err, map[string]any{"videoId": videoId, "filePath": video.FilePath})
		utils.WriteError(w, http.StatusInternalServerError, "failed to read file metadata")
		return
	}

	meta = &tor.FileMetadata{
		Name:      filepath.Base(video.FilePath),
		Path:      video.FilePath,
		Length:    info.Size(),
		Extension: filepath.Ext(video.FilePath),
		IsVideo:   utils.IsVideoFile(filepath.Ext(video.FilePath)),
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

	inputPath := video.FilePath
	if utils.FileExists(video.FilePath + ".part") {
		inputPath = video.FilePath + ".part"
	}

	if !utils.FileExists(inputPath) {
		err := fmt.Errorf("video file not found on disk: %s", inputPath)
		utils.LogHandlerError(r, "serveSubtitles", err, map[string]any{"videoId": videoId, "inputPath": inputPath})
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