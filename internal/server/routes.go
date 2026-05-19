package server

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

func (s *Server) RegisterRoutes() http.Handler {
	r := mux.NewRouter()

	r.Use(s.corsMiddleware)

	r.HandleFunc("/", s.HelloWorldHandler).Methods("GET", "OPTIONS")

	video := r.PathPrefix("/videos").Subrouter()
	torrent := r.PathPrefix("/torrents").Subrouter()
	video.HandleFunc("", s.createVideo).Methods("POST", "OPTIONS")
	video.HandleFunc("", s.listVideos).Methods("GET", "OPTIONS")
	video.HandleFunc("/{videoId}/metadata", s.getVideoMetadata).Methods("GET", "OPTIONS")
	video.HandleFunc("/{videoId}/stream", s.streamVideo).Methods("GET", "HEAD", "OPTIONS")
	torrent.HandleFunc("/{videoId}/stats/stream", s.torrentStatsStream).Methods("GET")
	torrent.HandleFunc("/{videoId}", s.deleteTorrent).Methods("DELETE")

	return r
}

// CORS middleware
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // Wildcard allows all origins
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "false") // Credentials not allowed with wildcard origins

		// Handle preflight OPTIONS
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) HelloWorldHandler(w http.ResponseWriter, r *http.Request) {
	resp := make(map[string]string)
	resp["message"] = "Hello World"

	jsonResp, err := json.Marshal(resp)
	if err != nil {
		log.Fatalf("error handling JSON marshal. Err: %v", err)
	}

	_, _ = w.Write(jsonResp)
}
