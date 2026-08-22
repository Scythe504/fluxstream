package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/scythe504/fluxstream/internal/utils"
)

func (s *Server) torrentStatsStream(w http.ResponseWriter, r *http.Request) {
	videoId := mux.Vars(r)["videoId"]

	flusher, ok := w.(http.Flusher)
	if !ok {
		err := fmt.Errorf("response writer does not support flushing")
		utils.LogHandlerError(r, "torrentStatsStream", err, map[string]any{"videoId": videoId})
		utils.WriteError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var prevDown, prevUp int64

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			stats, currentDown, currentUp, err := s.t.GetStats(videoId, prevDown, prevUp)
			if err != nil {
				utils.LogHandlerError(r, "torrentStatsStream", err, map[string]any{"videoId": videoId})
				fmt.Fprintf(w, "data: {\"error\": \"%s\"}\n\n", err.Error())
				flusher.Flush()
				continue
			}

			prevDown = currentDown
			prevUp = currentUp

			data, err := json.Marshal(stats)
			if err != nil {
				continue
			}

			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Server) deleteTorrent(w http.ResponseWriter, r *http.Request) {
	videoId := mux.Vars(r)["videoId"]

	var body struct {
		DeleteResource bool `json:"delete_resource"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.LogHandlerError(r, "deleteTorrent", err, map[string]any{"videoId": videoId})
		utils.WriteError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if body.DeleteResource {
		video, err := s.db.GetVideo(videoId)
		if err != nil {
			utils.LogHandlerError(r, "deleteTorrent", err, map[string]any{"videoId": videoId})
			utils.WriteError(w, http.StatusNotFound, "video not found")
			return
		}

		if actualPath, err := s.resolveVideoFilePath(videoId, video); err == nil {
			cleanPath := strings.TrimSuffix(actualPath, ".part")
			if err := os.Remove(cleanPath); err != nil && !os.IsNotExist(err) {
				utils.LogHandlerError(r, "deleteTorrent", err, map[string]any{"videoId": videoId, "filePath": cleanPath})
			}
			if err := os.Remove(cleanPath + ".part"); err != nil && !os.IsNotExist(err) {
				utils.LogHandlerError(r, "deleteTorrent", err, map[string]any{"videoId": videoId, "partPath": cleanPath + ".part"})
			}
		} else if video.FilePath != "" {
			if err := os.Remove(video.FilePath); err != nil && !os.IsNotExist(err) {
				utils.LogHandlerError(r, "deleteTorrent", err, map[string]any{"videoId": videoId, "filePath": video.FilePath})
			}
			partPath := video.FilePath + ".part"
			if err := os.Remove(partPath); err != nil && !os.IsNotExist(err) {
				utils.LogHandlerError(r, "deleteTorrent", err, map[string]any{"videoId": videoId, "partPath": partPath})
			}
		}
	}

	if err := s.t.CleanupTorrent(videoId); err != nil {
		utils.LogHandlerError(r, "deleteTorrent", err, map[string]any{"videoId": videoId})
		utils.WriteError(w, http.StatusInternalServerError, "failed to cleanup torrent")
		return
	}

	if err := s.db.DeleteVideo(videoId); err != nil {
		utils.LogHandlerError(r, "deleteTorrent", err, map[string]any{"videoId": videoId})
	}

	w.WriteHeader(http.StatusNoContent)
}
