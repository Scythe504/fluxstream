package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
)

func (s *Server) torrentStatsStream(w http.ResponseWriter, r *http.Request) {
	videoId := mux.Vars(r)["videoId"]

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
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
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if body.DeleteResource {
		video, err := s.db.GetVideo(videoId)
		if err != nil {
			http.Error(w, "video not found", http.StatusNotFound)
			return
		}

		if err := os.Remove(video.FilePath); err != nil && os.IsNotExist(err) {
			log.Println("[DeleteTorrent] failed to remove file: %v", err)
		}
	}

	if err := s.t.CleanupTorrent(videoId); err != nil {
		http.Error(w, "failed to cleanup torrent", http.StatusInternalServerError)
		return
	}

	if err := s.db.DeleteVideo(videoId); err != nil {
		log.Printf("[deleteTorrent] failed to delete db record: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
}
