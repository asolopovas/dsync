package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockDBProvider struct {
	DumpRemoteFunc          func(context.Context) (*DBDump, error)
	DumpLocalFunc           func(context.Context) (*DBDump, error)
	WriteRemoteFunc         func(context.Context, io.Reader) error
	WriteLocalFunc          func(context.Context, io.Reader) error
	BackupRemoteFunc        func(context.Context) error
	PreflightLocalCacheFunc func(context.Context, []string) error
	FlushLocalCacheFunc     func(context.Context, []string) error

	Calls []string
}

func (m *mockDBProvider) DumpRemote(ctx context.Context) (*DBDump, error) {
	m.Calls = append(m.Calls, "DumpRemote")
	if m.DumpRemoteFunc != nil {
		return m.DumpRemoteFunc(ctx)
	}
	return stringDump(""), nil
}

func (m *mockDBProvider) DumpLocal(ctx context.Context) (*DBDump, error) {
	m.Calls = append(m.Calls, "DumpLocal")
	if m.DumpLocalFunc != nil {
		return m.DumpLocalFunc(ctx)
	}
	return stringDump(""), nil
}

func (m *mockDBProvider) WriteRemote(ctx context.Context, sql io.Reader) error {
	m.Calls = append(m.Calls, "WriteRemote")
	if m.WriteRemoteFunc != nil {
		return m.WriteRemoteFunc(ctx, sql)
	}
	_, err := io.Copy(io.Discard, sql)
	return err
}

func (m *mockDBProvider) WriteLocal(ctx context.Context, sql io.Reader) error {
	m.Calls = append(m.Calls, "WriteLocal")
	if m.WriteLocalFunc != nil {
		return m.WriteLocalFunc(ctx, sql)
	}
	_, err := io.Copy(io.Discard, sql)
	return err
}

func (m *mockDBProvider) BackupRemote(ctx context.Context) error {
	m.Calls = append(m.Calls, "BackupRemote")
	if m.BackupRemoteFunc != nil {
		return m.BackupRemoteFunc(ctx)
	}
	return nil
}

func (m *mockDBProvider) PreflightLocalCache(ctx context.Context, wordpressRoots []string) error {
	m.Calls = append(m.Calls, "PreflightLocalCache")
	if m.PreflightLocalCacheFunc != nil {
		return m.PreflightLocalCacheFunc(ctx, wordpressRoots)
	}
	return nil
}

func (m *mockDBProvider) FlushLocalCache(ctx context.Context, wordpressRoots []string) error {
	m.Calls = append(m.Calls, "FlushLocalCache")
	if m.FlushLocalCacheFunc != nil {
		return m.FlushLocalCacheFunc(ctx, wordpressRoots)
	}
	return nil
}

func stringDump(sql string) *DBDump {
	return &DBDump{
		Reader: io.NopCloser(strings.NewReader(sql)),
		Wait:   func() error { return nil },
	}
}

func dbSyncConfig(replacements ...DBReplace) *Config {
	return &Config{
		Remote:    HostSettings{DB: "remote_db"},
		Local:     HostSettings{DB: "local_db"},
		DBReplace: replacements,
	}
}

func requireCalls(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %v, want %v", got, want)
	}
}

func TestSyncDBForward(t *testing.T) {
	mock := &mockDBProvider{
		DumpRemoteFunc: func(context.Context) (*DBDump, error) {
			return stringDump("INSERT INTO users VALUES ('remote');"), nil
		},
		WriteLocalFunc: func(_ context.Context, sql io.Reader) error {
			assertStringEqual(t, "local import SQL", readStringForTest(t, sql), "INSERT INTO users VALUES ('remote');")
			return nil
		},
	}

	if err := SyncDB(context.Background(), mock, dbSyncConfig(), false, false); err != nil {
		t.Fatalf("SyncDB failed: %v", err)
	}
	requireCalls(t, mock.Calls, []string{"DumpRemote", "WriteLocal"})
}

func TestLocalWordPressRootsDiscoversAndDeduplicatesSites(t *testing.T) {
	firstRoot := filepath.Join(t.TempDir(), "first-site")
	secondRoot := filepath.Join(t.TempDir(), "second-site")
	cfg := &Config{Sync: []SyncPath{
		{Local: filepath.Join(firstRoot, "wp-content", "plugins")},
		{Local: filepath.Join(firstRoot, "wp-content", "uploads")},
		{Local: filepath.Join(secondRoot, "WP-CONTENT", "themes")},
		{Local: filepath.Join(t.TempDir(), "assets")},
	}}

	roots, err := localWordPressRoots(cfg)
	if err != nil {
		t.Fatalf("localWordPressRoots() error = %v", err)
	}

	want := map[string]bool{firstRoot: true, secondRoot: true}
	if len(roots) != len(want) {
		t.Fatalf("localWordPressRoots() returned %v", roots)
	}
	for _, root := range roots {
		if !want[root] {
			t.Fatalf("localWordPressRoots() returned unexpected root %q", root)
		}
	}
}

func TestLocalWordPressRootsIgnoresNonWordPressPaths(t *testing.T) {
	cfg := &Config{Sync: []SyncPath{
		{Local: filepath.Join(t.TempDir(), "content", "uploads")},
		{Local: filepath.Join(t.TempDir(), "wp-content-backup", "themes")},
	}}

	roots, err := localWordPressRoots(cfg)
	if err != nil {
		t.Fatalf("localWordPressRoots() error = %v", err)
	}
	if len(roots) != 0 {
		t.Fatalf("localWordPressRoots() returned %v for non-WordPress paths", roots)
	}
}

func TestSyncDBForwardPreflightsAndFlushesWordPressCache(t *testing.T) {
	root := filepath.Join(t.TempDir(), "site")
	cfg := dbSyncConfig()
	cfg.Sync = []SyncPath{
		{Local: filepath.Join(root, "wp-content", "plugins")},
		{Local: filepath.Join(root, "wp-content", "uploads")},
	}
	var preflightRoots []string
	var flushedRoots []string
	mock := &mockDBProvider{
		PreflightLocalCacheFunc: func(_ context.Context, roots []string) error {
			preflightRoots = append(preflightRoots, roots...)
			return nil
		},
		DumpRemoteFunc: func(context.Context) (*DBDump, error) {
			return stringDump("SELECT 1;"), nil
		},
		FlushLocalCacheFunc: func(_ context.Context, roots []string) error {
			flushedRoots = append(flushedRoots, roots...)
			return nil
		},
	}

	if err := SyncDB(context.Background(), mock, cfg, false, false); err != nil {
		t.Fatalf("SyncDB failed: %v", err)
	}
	requireCalls(t, mock.Calls, []string{"PreflightLocalCache", "DumpRemote", "WriteLocal", "FlushLocalCache"})
	if len(preflightRoots) != 1 || preflightRoots[0] != root {
		t.Fatalf("preflight roots = %v, want %q once", preflightRoots, root)
	}
	if len(flushedRoots) != 1 || flushedRoots[0] != root {
		t.Fatalf("flushed roots = %v, want %q once", flushedRoots, root)
	}
}

func TestSyncDBForwardStopsBeforeImportWhenWordPressPreflightFails(t *testing.T) {
	cfg := dbSyncConfig()
	cfg.Sync = []SyncPath{{Local: filepath.Join(t.TempDir(), "site", "wp-content", "uploads")}}
	mock := &mockDBProvider{
		PreflightLocalCacheFunc: func(context.Context, []string) error {
			return errors.New("wp executable unavailable")
		},
	}

	err := SyncDB(context.Background(), mock, cfg, false, false)
	if err == nil || !strings.Contains(err.Error(), "before local database import") {
		t.Fatalf("SyncDB error = %v", err)
	}
	requireCalls(t, mock.Calls, []string{"PreflightLocalCache"})
}

func TestSyncDBForwardDoesNotFlushCacheAfterFailedImport(t *testing.T) {
	cfg := dbSyncConfig()
	cfg.Sync = []SyncPath{{Local: filepath.Join(t.TempDir(), "site", "wp-content", "uploads")}}
	mock := &mockDBProvider{
		DumpRemoteFunc: func(context.Context) (*DBDump, error) {
			return stringDump("SELECT 1;"), nil
		},
		WriteLocalFunc: func(context.Context, io.Reader) error {
			return errors.New("import failed")
		},
	}

	err := SyncDB(context.Background(), mock, cfg, false, false)
	if err == nil || !strings.Contains(err.Error(), "failed to write to local db") {
		t.Fatalf("SyncDB error = %v", err)
	}
	requireCalls(t, mock.Calls, []string{"PreflightLocalCache", "DumpRemote", "WriteLocal"})
}

func TestSyncDBForwardReportsImportedDatabaseWhenCacheFlushFails(t *testing.T) {
	cfg := dbSyncConfig()
	cfg.Sync = []SyncPath{{Local: filepath.Join(t.TempDir(), "site", "wp-content", "uploads")}}
	mock := &mockDBProvider{
		DumpRemoteFunc: func(context.Context) (*DBDump, error) {
			return stringDump("SELECT 1;"), nil
		},
		FlushLocalCacheFunc: func(context.Context, []string) error {
			return errors.New("redis flush denied")
		},
	}

	err := SyncDB(context.Background(), mock, cfg, false, false)
	if err == nil {
		t.Fatal("SyncDB succeeded despite cache flush failure")
	}
	for _, message := range []string{"local database 'local_db' imported successfully", "cache invalidation failed", "redis flush denied"} {
		if !strings.Contains(err.Error(), message) {
			t.Fatalf("SyncDB error %q does not contain %q", err, message)
		}
	}
	requireCalls(t, mock.Calls, []string{"PreflightLocalCache", "DumpRemote", "WriteLocal", "FlushLocalCache"})
}

func TestSyncDBReverseBacksUpThenImportsTransformedDump(t *testing.T) {
	replacements := []DBReplace{
		{From: "example.com", To: "example.test"},
		{From: "https://example.test", To: "http://example.test"},
	}
	mock := &mockDBProvider{
		DumpLocalFunc: func(context.Context) (*DBDump, error) {
			return stringDump("Check http://example.test now"), nil
		},
		BackupRemoteFunc: func(context.Context) error { return nil },
		WriteRemoteFunc: func(_ context.Context, sql io.Reader) error {
			assertStringEqual(t, "remote import SQL", readStringForTest(t, sql), "Check https://example.com now")
			return nil
		},
	}

	cfg := dbSyncConfig(replacements...)
	cfg.Sync = []SyncPath{{Local: filepath.Join(t.TempDir(), "site", "wp-content", "uploads")}}
	cfg.DBReplaceEngine = DBReplaceEngineRaw
	if err := SyncDB(context.Background(), mock, cfg, false, true); err != nil {
		t.Fatalf("SyncDB failed: %v", err)
	}
	requireCalls(t, mock.Calls, []string{"DumpLocal", "BackupRemote", "WriteRemote"})
}

func TestRealDBProviderCachePreflightRequiresWordPressRoot(t *testing.T) {
	provider := NewRealDBProvider(&Config{})
	missingRoot := filepath.Join(t.TempDir(), "missing-site")

	err := provider.PreflightLocalCache(context.Background(), []string{missingRoot})
	if err == nil || !strings.Contains(err.Error(), missingRoot) {
		t.Fatalf("PreflightLocalCache() error = %v", err)
	}
}

func TestRealDBProviderCachePreflightRequiresWPCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "wp-load.php"), []byte("<?php"), 0644); err != nil {
		t.Fatalf("write wp-load.php: %v", err)
	}
	t.Setenv("PATH", t.TempDir())
	provider := NewRealDBProvider(&Config{})

	err := provider.PreflightLocalCache(context.Background(), []string{root})
	if err == nil || !strings.Contains(err.Error(), "WP-CLI executable 'wp' is unavailable") {
		t.Fatalf("PreflightLocalCache() error = %v", err)
	}
}

func TestRealDBProviderFlushesEachWordPressCacheWithSafeBootstrap(t *testing.T) {
	binDir := t.TempDir()
	argsPath := filepath.Join(t.TempDir(), "wp-args")
	wpPath := filepath.Join(binDir, "wp")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$DSYNC_WP_ARGS\"\n"
	if err := os.WriteFile(wpPath, []byte(script), 0755); err != nil {
		t.Fatalf("write wp executable: %v", err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("DSYNC_WP_ARGS", argsPath)

	var roots []string
	for range 2 {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "wp-load.php"), []byte("<?php"), 0644); err != nil {
			t.Fatalf("write wp-load.php: %v", err)
		}
		roots = append(roots, root)
	}
	provider := NewRealDBProvider(&Config{})
	if err := provider.PreflightLocalCache(context.Background(), roots); err != nil {
		t.Fatalf("PreflightLocalCache() error = %v", err)
	}
	if err := provider.FlushLocalCache(context.Background(), roots); err != nil {
		t.Fatalf("FlushLocalCache() error = %v", err)
	}

	data, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read wp arguments: %v", err)
	}
	for _, root := range roots {
		invocation := strings.Join([]string{"--path=" + root, "--skip-plugins", "--skip-themes", "--quiet", "cache", "flush"}, "\n") + "\n"
		if strings.Count(string(data), invocation) != 1 {
			t.Fatalf("wp arguments %q do not contain one invocation for %q", data, root)
		}
	}
}

func TestRealDBProviderCacheFlushPreservesCommandOutput(t *testing.T) {
	binDir := t.TempDir()
	wpPath := filepath.Join(binDir, "wp")
	script := "#!/bin/sh\nprintf 'redis flush denied\\n' >&2\nexit 7\n"
	if err := os.WriteFile(wpPath, []byte(script), 0755); err != nil {
		t.Fatalf("write wp executable: %v", err)
	}
	t.Setenv("PATH", binDir)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "wp-load.php"), []byte("<?php"), 0644); err != nil {
		t.Fatalf("write wp-load.php: %v", err)
	}
	provider := NewRealDBProvider(&Config{})
	if err := provider.PreflightLocalCache(context.Background(), []string{root}); err != nil {
		t.Fatalf("PreflightLocalCache() error = %v", err)
	}

	err := provider.FlushLocalCache(context.Background(), []string{root})
	if err == nil || !strings.Contains(err.Error(), "redis flush denied") {
		t.Fatalf("FlushLocalCache() error = %v", err)
	}
}

type closeSignalReadCloser struct {
	reader io.Reader
	once   sync.Once
	closed chan struct{}
}

func newCloseSignalReadCloser(value string) *closeSignalReadCloser {
	return &closeSignalReadCloser{
		reader: strings.NewReader(value),
		closed: make(chan struct{}),
	}
}

func (r *closeSignalReadCloser) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func (r *closeSignalReadCloser) Close() error {
	r.once.Do(func() { close(r.closed) })
	return nil
}

func TestWriteTransformedDumpStopsDumpOnTransformError(t *testing.T) {
	reader := newCloseSignalReadCloser("INSERT INTO t (`v`) VALUES ('x';")
	dump := &DBDump{
		Reader: reader,
		Wait: func() error {
			select {
			case <-reader.closed:
				return nil
			case <-time.After(time.Second):
				return context.DeadlineExceeded
			}
		},
	}

	err := writeTransformedDump(
		context.Background(),
		dump,
		&Config{DBReplaceEngine: DBReplaceEngineGoSerialized},
		nil,
		false,
		"db.sql",
		func(_ context.Context, sql io.Reader) error {
			_, err := io.Copy(io.Discard, sql)
			return err
		},
		&dbStreamProgress{},
	)
	if err == nil {
		t.Fatal("expected transform error")
	}
	if !strings.Contains(err.Error(), "parse INSERT values") {
		t.Fatalf("expected transform error, got %v", err)
	}
}
