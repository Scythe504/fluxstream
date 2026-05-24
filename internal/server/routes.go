package server

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/gorilla/mux"
	"github.com/scythe504/fluxstream/internal/provider"
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
	providers := r.PathPrefix("/providers")
	providers.PathPrefix("/{provider}/").HandlerFunc(s.reverseProxyProvider)

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

func (s *Server) reverseProxyProvider(w http.ResponseWriter, r *http.Request) {
	providerName := mux.Vars(r)["provider"]
	if providerName == "" {
		http.Error(w, "provider missing", http.StatusBadRequest)
		return
	}

	// strip /plugins/{provider} prefix to get the path to forward
	providerPath := strings.TrimPrefix(r.URL.Path, "/providers/"+providerName)

	log.Println("Path: ", providerPath)

	p, ok := s.providers.Load(providerName)
	if !ok {
		pr := provider.InitProvider(providerName, "http://localhost:8081")
		if pr == nil {
			http.Error(w, "failed to init provider", http.StatusBadGateway)
			return
		}
		s.providers.Store(providerName, pr)
		p = pr
	}

	prov := p.(*provider.Provider)
	prov.Proxy.Rewrite = func(req *httputil.ProxyRequest) {
		req.SetURL(prov.BaseUrl)
		req.Out.URL.Path = providerPath
		req.Out.URL.RawQuery = req.In.URL.RawQuery
	}

	prov.Proxy.ServeHTTP(w, r)
}
