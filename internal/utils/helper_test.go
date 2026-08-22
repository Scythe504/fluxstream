package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetDownloadDir(t *testing.T) {
	// Test environment override
	customPath := "/custom/download/path"
	t.Setenv("DOWNLOAD_PATH", customPath)
	if dir := GetDownloadDir(); dir != customPath {
		t.Errorf("expected %s, got %s", customPath, dir)
	}

	// Test fallback when env is unset
	t.Setenv("DOWNLOAD_PATH", "")
	dir := GetDownloadDir()
	if dir == "" {
		t.Errorf("expected non-empty default download directory, got empty string")
	}
}

func TestFileExists(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "fluxstream_test_*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	if !FileExists(tmpFile.Name()) {
		t.Errorf("expected FileExists to return true for %s", tmpFile.Name())
	}

	nonExistent := filepath.Join(os.TempDir(), "fluxstream_non_existent_file_12345.txt")
	if FileExists(nonExistent) {
		t.Errorf("expected FileExists to return false for %s", nonExistent)
	}
}
