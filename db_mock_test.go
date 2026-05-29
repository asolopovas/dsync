package main

import (
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type MockDBProvider struct {
	DumpRemoteFunc   func(ctx context.Context) (*DBDump, error)
	DumpLocalFunc    func(ctx context.Context) (*DBDump, error)
	WriteRemoteFunc  func(ctx context.Context, sql io.Reader) error
	WriteLocalFunc   func(ctx context.Context, sql io.Reader) error
	BackupRemoteFunc func(ctx context.Context) error

	Calls []string
}

func (m *MockDBProvider) DumpRemote(ctx context.Context) (*DBDump, error) {
	m.Calls = append(m.Calls, "DumpRemote")
	if m.DumpRemoteFunc != nil {
		return m.DumpRemoteFunc(ctx)
	}
	return stringDump(""), nil
}

func (m *MockDBProvider) DumpLocal(ctx context.Context) (*DBDump, error) {
	m.Calls = append(m.Calls, "DumpLocal")
	if m.DumpLocalFunc != nil {
		return m.DumpLocalFunc(ctx)
	}
	return stringDump(""), nil
}

func (m *MockDBProvider) WriteRemote(ctx context.Context, sql io.Reader) error {
	m.Calls = append(m.Calls, "WriteRemote")
	if m.WriteRemoteFunc != nil {
		return m.WriteRemoteFunc(ctx, sql)
	}
	return nil
}

func (m *MockDBProvider) WriteLocal(ctx context.Context, sql io.Reader) error {
	m.Calls = append(m.Calls, "WriteLocal")
	if m.WriteLocalFunc != nil {
		return m.WriteLocalFunc(ctx, sql)
	}
	return nil
}

func (m *MockDBProvider) BackupRemote(ctx context.Context) error {
	m.Calls = append(m.Calls, "BackupRemote")
	if m.BackupRemoteFunc != nil {
		return m.BackupRemoteFunc(ctx)
	}
	return nil
}

func stringDump(sql string) *DBDump {
	return &DBDump{
		Reader: io.NopCloser(strings.NewReader(sql)),
		Wait: func() error {
			return nil
		},
	}
}

func readAllForTest(t *testing.T, reader io.Reader) string {
	t.Helper()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read sql: %v", err)
	}
	return string(data)
}

func TestSyncDB_Forward(t *testing.T) {
	mock := &MockDBProvider{
		DumpRemoteFunc: func(ctx context.Context) (*DBDump, error) {
			return stringDump("INSERT INTO users VALUES ('remote');"), nil
		},
		WriteLocalFunc: func(ctx context.Context, sql io.Reader) error {
			got := readAllForTest(t, sql)
			if got != "INSERT INTO users VALUES ('remote');" {
				t.Errorf("Unexpected SQL: %s", got)
			}
			return nil
		},
	}

	cfg := &Config{
		Remote: HostSettings{DB: "remote_db"},
		Local:  HostSettings{DB: "local_db"},
	}

	err := SyncDB(context.Background(), mock, cfg, false, false)
	if err != nil {
		t.Fatalf("SyncDB failed: %v", err)
	}

	expectedCalls := []string{"DumpRemote", "WriteLocal"}
	if len(mock.Calls) != len(expectedCalls) {
		t.Errorf("Expected calls %v, got %v", expectedCalls, mock.Calls)
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
		func(ctx context.Context, sql io.Reader) error {
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

func TestSyncDB_Reverse(t *testing.T) {
	mock := &MockDBProvider{
		DumpLocalFunc: func(ctx context.Context) (*DBDump, error) {
			return stringDump("INSERT INTO users VALUES ('local');"), nil
		},
		BackupRemoteFunc: func(ctx context.Context) error {
			return nil
		},
		WriteRemoteFunc: func(ctx context.Context, sql io.Reader) error {
			got := readAllForTest(t, sql)
			if got != "INSERT INTO users VALUES ('local');" {
				t.Errorf("Unexpected SQL: %s", got)
			}
			return nil
		},
	}

	cfg := &Config{
		Remote: HostSettings{DB: "remote_db"},
		Local:  HostSettings{DB: "local_db"},
	}

	err := SyncDB(context.Background(), mock, cfg, false, true)
	if err != nil {
		t.Fatalf("SyncDB failed: %v", err)
	}

	expectedCalls := []string{"DumpLocal", "BackupRemote", "WriteRemote"}
	if len(mock.Calls) != len(expectedCalls) {
		t.Errorf("Expected calls %v, got %v", expectedCalls, mock.Calls)
	}

	// Verify order specifically
	if mock.Calls[1] != "BackupRemote" {
		t.Error("BackupRemote must be called before WriteRemote")
	}
}
