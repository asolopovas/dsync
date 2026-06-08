package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyFileReplacementsUpdatesTextFilesOnly(t *testing.T) {
	tmp := t.TempDir()

	cssPath := filepath.Join(tmp, "style.css")
	if err := os.WriteFile(cssPath, []byte("body{background:url(https://example.com/image.jpg)}"), 0644); err != nil {
		t.Fatalf("write css fixture: %v", err)
	}

	fontPath := filepath.Join(tmp, "font.woff2")
	if err := os.WriteFile(fontPath, []byte("https://example.com"), 0644); err != nil {
		t.Fatalf("write binary fixture: %v", err)
	}

	changed, err := applyFileReplacements(tmp, []DBReplace{{From: "https://example.com", To: "http://example.test"}})
	if err != nil {
		t.Fatalf("applyFileReplacements() error = %v", err)
	}
	if changed != 1 {
		t.Fatalf("applyFileReplacements() changed = %d, want 1", changed)
	}

	css, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatalf("read css result: %v", err)
	}
	if got, want := string(css), "body{background:url(http://example.test/image.jpg)}"; got != want {
		t.Fatalf("css result = %q, want %q", got, want)
	}

	font, err := os.ReadFile(fontPath)
	if err != nil {
		t.Fatalf("read binary result: %v", err)
	}
	if got, want := string(font), "https://example.com"; got != want {
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
