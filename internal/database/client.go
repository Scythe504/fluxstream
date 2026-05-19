package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// Service represents a service that interacts with a database.
type Service interface {
	// VideoMethods
	GetVideo(videoId string) (Video, error)
	CreateVideo(video Video) error
	GetAllVideos() ([]Video, error)
	DeleteVideo(videoId string) error
	// Health returns a map of health status information.
	// The keys and values in the map are service-specific.
	Health() map[string]string
	// Close terminates the database connection.
	// It returns an error if the connection cannot be closed.
	Close() error
}

type service struct {
	db *sql.DB
}

var (
	dbUrl = os.Getenv("DB_URL")
)

func New() Service {
	if dbUrl == "" {
		dbUrl = "fluxstream.db"
	}

	db, err := sql.Open("sqlite", dbUrl)
	if err != nil {
		log.Fatal(err)
	}

	// Performance pragmas
	_, _ = db.Exec("PRAGMA journal_mode=WAL")
	_, _ = db.Exec("PRAGMA synchronous=NORMAL")

	s := &service{db: db}

	// Run migrations
	if err := s.migrate(); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	return s
}

func (s *service) migrate() error {
	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}

	if err := goose.Up(s.db, "migrations"); err != nil {
		return err
	}

	return nil
}

// Health checks the health of the database connection by pinging the database.
// It returns a map with keys indicating various health statistics.
func (s *service) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	stats := make(map[string]string)

	// Ping the database
	err := s.db.PingContext(ctx)
	if err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("db down: %v", err)
		return stats
	}

	// Database is up, add more statistics
	stats["status"] = "up"
	stats["message"] = "It's healthy"

	// Get database stats (like open connections, in use, idle, etc.)
	dbStats := s.db.Stats()
	stats["open_connections"] = strconv.Itoa(dbStats.OpenConnections)
	stats["in_use"] = strconv.Itoa(dbStats.InUse)
	stats["idle"] = strconv.Itoa(dbStats.Idle)
	stats["wait_count"] = strconv.FormatInt(dbStats.WaitCount, 10)
	stats["wait_duration"] = dbStats.WaitDuration.String()
	stats["max_idle_closed"] = strconv.FormatInt(dbStats.MaxIdleClosed, 10)
	stats["max_lifetime_closed"] = strconv.FormatInt(dbStats.MaxLifetimeClosed, 10)

	return stats
}

// Close closes the database connection.
func (s *service) Close() error {
	log.Printf("Disconnected from database: %s", dbUrl)
	return s.db.Close()
}
