package server

import (
	_ "context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	_ "github.com/joho/godotenv/autoload"
	postgresdb "github.com/scythe504/fluxstream/internal/postgres-db"
	redisdb "github.com/scythe504/fluxstream/internal/redis-db"
	"github.com/scythe504/fluxstream/internal/tor"
)

type StreamResolver struct {
	cache sync.Map
}

type Server struct {
	port           int
	rdb            redisdb.Service
	db             postgresdb.Service
	t              tor.Torrent
	streamResolver *StreamResolver
}

func NewServer() *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	// ctx := context.Background()
	NewServer := &Server{
		port: port,
		// rdb:  redisdb.New(ctx),
		// db:   postgresdb.New(),
		t:              tor.New(42069),
		streamResolver: &StreamResolver{cache: sync.Map{}},
	}

	// Declare Server config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", NewServer.port),
		Handler:      NewServer.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}
