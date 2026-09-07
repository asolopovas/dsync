package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestReplacementExclusionsAgreeWithRsync(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync unavailable")
	}
	paths := []string{"root.txt", "a/root.txt", "a/b/root.txt", "cache/c.txt", "a/cache/c.txt", "a/b/cache/c.txt", "a/b/keep.txt", "a/keep.txt", "z/a/keep.txt", "z/a/b/keep.txt", `foo\bar.txt`, "line\nfile.txt", "a/dataa.txt", "a/data].txt", "a/data!.txt", "a/data1.txt", "a/data2.txt", "a/datax.txt", "cache.txt", "literal[.txt", "a/file.css"}
	for _, patterns := range [][]string{{"/root.txt"}, {"root.txt"}, {"cache/"}, {"/cache/"}, {"a/*.txt"}, {"**/root.txt"}, {"a/**/keep.txt"}, {"a/***"}, {"cache/***"}, {"data[12].txt"}, {"data[!12].txt"}, {"*.css"}, {"/a/cache/", "data?.txt"}, {"literal\\[.txt"}, {"data[[:alpha:]].txt"}, {`foo\bar.txt`}, {"line*.txt"}, {"a/**/keep.txt"}, {"**/cache/***"}, {"/a/***"}, {"a**keep.txt"}, {"+ root.txt", "*.txt"}, {"*.txt", "!", "*.css"}, {"- root.txt"}, {"data[!]].txt"}, {"data[]].txt"}} {
		t.Run(strings.Join(patterns, "+"), func(t *testing.T) {
			source, destination := t.TempDir(), t.TempDir()
			for _, path := range paths {
				full := filepath.Join(source, path)
				if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, full, "remote")
			}
			args := []string{"-r"}
			for _, pattern := range patterns {
				args = append(args, "--exclude="+pattern)
			}
			args = append(args, "--", source+"/", destination+"/")
			if out, err := exec.CommandContext(t.Context(), "rsync", args...).CombinedOutput(); err != nil {
				t.Fatalf("rsync: %s %v", out, err)
			}
			if _, err := applyFileReplacementsContext(t.Context(), source, []DBReplace{{From: "remote", To: "local"}}, patterns); err != nil {
				t.Fatal(err)
			}
			for _, path := range paths {
				_, err := os.Stat(filepath.Join(destination, path))
				included := err == nil
				changed := readTestFile(t, filepath.Join(source, path)) == "local"
				if included != changed {
					t.Errorf("%s: rsync included=%v replacement changed=%v", path, included, changed)
				}
			}
		})
	}
}

func TestReplacementPreservesMetadataAndSkipsLinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "target.css")
	writeTestFile(t, target, "remote")
	link := filepath.Join(dir, "link.css")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "style.css")
	writeTestFile(t, path, "remote")
	stamp := time.Unix(1234567890, 0)
	if err := os.Chmod(path, 0751); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(path)
	changed, err := applyFileReplacementsContext(t.Context(), dir, []DBReplace{{From: "remote", To: "local"}}, nil)
	if err != nil || changed != 1 {
		t.Fatalf("changed=%d err=%v", changed, err)
	}
	after, _ := os.Stat(path)
	if after.Mode() != before.Mode() || !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("metadata changed")
	}
	if os.SameFile(before, after) {
		t.Fatal("replacement was not atomic")
	}
	if readTestFile(t, target) != "remote" {
		t.Fatal("symlink target changed")
	}
	// A symlink used as the traversal root must not be followed either.
	rootLink := filepath.Join(t.TempDir(), "root")
	if err := os.Symlink(filepath.Dir(target), rootLink); err != nil {
		t.Fatal(err)
	}
	if changed, err := applyFileReplacementsContext(t.Context(), rootLink+"/", []DBReplace{{From: "remote", To: "local"}}, nil); err != nil || changed != 0 {
		t.Fatalf("root symlink: %d %v", changed, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := applyFileReplacementsContext(ctx, dir, []DBReplace{{From: "local", To: "changed"}}, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if readTestFile(t, path) != "local" {
		t.Fatal("modified after cancel")
	}
}
