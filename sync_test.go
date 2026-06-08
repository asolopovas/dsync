package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestApplyFileReplacementsUpdatesTextFilesOnly(t *testing.T) {
	tmp := t.TempDir()
	cssPath := filepath.Join(tmp, "style.css")
	fontPath := filepath.Join(tmp, "font.woff2")

	writeTestFile(t, cssPath, "body{background:url(https://example.com/image.jpg)}")
	writeTestFile(t, fontPath, "https://example.com")

	changed, err := applyFileReplacements(tmp, []DBReplace{{From: "https://example.com", To: "http://example.test"}})
	if err != nil {
		t.Fatalf("applyFileReplacements() error = %v", err)
	}
	if changed != 1 {
		t.Fatalf("applyFileReplacements() changed = %d, want 1", changed)
	}
	if got, want := readTestFile(t, cssPath), "body{background:url(http://example.test/image.jpg)}"; got != want {
		t.Fatalf("css result = %q, want %q", got, want)
	}
	if got, want := readTestFile(t, fontPath), "https://example.com"; got != want {
		t.Fatalf("binary result = %q, want %q", got, want)
	}
}

func TestEnsureTrailingSlash(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"empty", "", "/"},
		{"no slash", "/path/to/dir", "/path/to/dir/"},
		{"with slash", "/path/to/dir/", "/path/to/dir/"},
		{"root", "/", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ensureTrailingSlash(tt.path); got != tt.want {
				t.Errorf("ensureTrailingSlash() = %v, want %v", got, tt.want)
			}
		})
	}
}
