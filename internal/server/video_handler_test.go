package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/scythe504/fluxstream/internal/database"
)

func TestResolveVideoFilePath(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOWNLOAD_PATH", tempDir)

	s := &Server{}
	// Note: s.t dataDir is resolved from DOWNLOAD_PATH if initialized, or we can test fallback paths

	// 1. Test direct file match
	directFile := filepath.Join(tempDir, "video1.mkv")
	if err := os.WriteFile(directFile, []byte("dummy"), 0644); err != nil {
		t.Fatalf("failed to create dummy file: %v", err)
	}

	video := database.Video{
		Id:       "test-id-1",
		FilePath: directFile,
	}
	resolved, err := s.resolveVideoFilePath(video.Id, video)
	if err != nil {
		t.Fatalf("unexpected error resolving direct file: %v", err)
	}
	if resolved != directFile {
		t.Errorf("expected %s, got %s", directFile, resolved)
	}

	// 2. Test .part file match
	partFile := filepath.Join(tempDir, "video2.mkv.part")
	if err := os.WriteFile(partFile, []byte("dummy"), 0644); err != nil {
		t.Fatalf("failed to create dummy part file: %v", err)
	}

	video2 := database.Video{
		Id:       "test-id-2",
		FilePath: filepath.Join(tempDir, "video2.mkv"),
	}
	resolved2, err := s.resolveVideoFilePath(video2.Id, video2)
	if err != nil {
		t.Fatalf("unexpected error resolving part file: %v", err)
	}
	if resolved2 != partFile {
		t.Errorf("expected %s, got %s", partFile, resolved2)
	}

	// 3. Test subfolder discovery when DB stored only the basename or wrong folder
	subDir := filepath.Join(tempDir, "Torrent Subfolder")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subfolder: %v", err)
	}
	nestedFile := filepath.Join(subDir, "episode_01.mkv.part")
	if err := os.WriteFile(nestedFile, []byte("dummy"), 0644); err != nil {
		t.Fatalf("failed to create nested file: %v", err)
	}

	video3 := database.Video{
		Id:       "test-id-3",
		FilePath: "episode_01.mkv", // DB only had basename
	}
	resolved3, err := s.resolveVideoFilePath(video3.Id, video3)
	if err != nil {
		t.Fatalf("unexpected error resolving nested file from basename: %v", err)
	}
	if resolved3 != nestedFile {
		t.Errorf("expected %s, got %s", nestedFile, resolved3)
	}

	// 4. Test non-existent file
	video4 := database.Video{
		Id:       "test-id-4",
		FilePath: "non_existent.mkv",
	}
	_, err = s.resolveVideoFilePath(video4.Id, video4)
	if err == nil {
		t.Errorf("expected error for non-existent file, got nil")
	}
}
