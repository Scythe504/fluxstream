package server

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/scythe504/fluxstream/internal/utils"
)

func (s *Server) RegisterRoutes() http.Handler {
	r := mux.NewRouter()

	r.Use(s.corsMiddleware)

	r.HandleFunc("/", s.HelloWorldHandler).Methods("GET", "OPTIONS")

	api := r.PathPrefix("/api").Subrouter()

	video := api.PathPrefix("/videos").Subrouter()
	torrent := api.PathPrefix("/torrents").Subrouter()
	providers := api.PathPrefix("/providers").Subrouter()

	video.HandleFunc("", s.createVideo).Methods("POST", "OPTIONS")
	video.HandleFunc("", s.listVideos).Methods("GET", "OPTIONS")
	video.HandleFunc("/{videoId}/metadata", s.getVideoMetadata).Methods("GET", "OPTIONS")
	video.HandleFunc("/{videoId}/stream", s.streamVideo).Methods("GET", "HEAD", "OPTIONS")
	video.HandleFunc("/{videoId}/subs", s.serveSubtitles).Methods("GET", "HEAD", "OPTIONS")
	torrent.HandleFunc("/{videoId}/stats/stream", s.torrentStatsStream).Methods("GET")
	torrent.HandleFunc("/{videoId}", s.deleteTorrent).Methods("DELETE", "OPTIONS")
	providers.HandleFunc("/", s.listVerifiedProviders).Methods("GET", "OPTIONS")
	providers.HandleFunc("", s.listVerifiedProviders).Methods("GET", "OPTIONS")
	providers.PathPrefix("/{provider}").HandlerFunc(s.reverseProxyProvider)

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
	resp := map[string]string{"message": "Hello World"}
	utils.WriteJSON(w, http.StatusOK, resp)
}