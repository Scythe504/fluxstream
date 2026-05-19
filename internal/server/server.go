package server

import (
	_ "context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/scythe504/fluxstream/internal/database"
	"github.com/scythe504/fluxstream/internal/tor"
)

type Server struct {
	port int
	db   database.Service
	t    tor.TorrentManager
}

func NewServer() *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	// ctx := context.Background()
	NewServer := &Server{
		port: port,
		db:   database.New(),
		t:    tor.New(42069),
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
