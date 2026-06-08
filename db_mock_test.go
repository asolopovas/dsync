package main

import (
	"context"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockDBProvider struct {
	DumpRemoteFunc   func(context.Context) (*DBDump, error)
	DumpLocalFunc    func(context.Context) (*DBDump, error)
	WriteRemoteFunc  func(context.Context, io.Reader) error
	WriteLocalFunc   func(context.Context, io.Reader) error
	BackupRemoteFunc func(context.Context) error

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
	return nil
}

func (m *mockDBProvider) WriteLocal(ctx context.Context, sql io.Reader) error {
	m.Calls = append(m.Calls, "WriteLocal")
	if m.WriteLocalFunc != nil {
		return m.WriteLocalFunc(ctx, sql)
	}
	return nil
}

func (m *mockDBProvider) BackupRemote(ctx context.Context) error {
	m.Calls = append(m.Calls, "BackupRemote")
	if m.BackupRemoteFunc != nil {
		return m.BackupRemoteFunc(ctx)
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

	if err := SyncDB(context.Background(), mock, dbSyncConfig(replacements...), false, true); err != nil {
		t.Fatalf("SyncDB failed: %v", err)
	}
	requireCalls(t, mock.Calls, []string{"DumpLocal", "BackupRemote", "WriteRemote"})
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
