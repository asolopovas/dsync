package main

import (
	"context"
	"testing"
)

type MockDBProvider struct {
	DumpRemoteFunc   func(ctx context.Context) (string, error)
	DumpLocalFunc    func(ctx context.Context) (string, error)
	WriteRemoteFunc  func(ctx context.Context, sql string) error
	WriteLocalFunc   func(ctx context.Context, sql string) error
	BackupRemoteFunc func(ctx context.Context) error

	Calls []string
}

func (m *MockDBProvider) DumpRemote(ctx context.Context) (string, error) {
	m.Calls = append(m.Calls, "DumpRemote")
	if m.DumpRemoteFunc != nil {
		return m.DumpRemoteFunc(ctx)
	}
	return "", nil
}

func (m *MockDBProvider) DumpLocal(ctx context.Context) (string, error) {
	m.Calls = append(m.Calls, "DumpLocal")
	if m.DumpLocalFunc != nil {
		return m.DumpLocalFunc(ctx)
	}
	return "", nil
}

func (m *MockDBProvider) WriteRemote(ctx context.Context, sql string) error {
	m.Calls = append(m.Calls, "WriteRemote")
	if m.WriteRemoteFunc != nil {
		return m.WriteRemoteFunc(ctx, sql)
	}
	return nil
}

func (m *MockDBProvider) WriteLocal(ctx context.Context, sql string) error {
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

func TestSyncDB_Forward(t *testing.T) {
	mock := &MockDBProvider{
		DumpRemoteFunc: func(ctx context.Context) (string, error) {
			return "INSERT INTO users VALUES ('remote');", nil
		},
		WriteLocalFunc: func(ctx context.Context, sql string) error {
			if sql != "INSERT INTO users VALUES ('remote');" {
				t.Errorf("Unexpected SQL: %s", sql)
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

func TestSyncDB_Reverse(t *testing.T) {
	mock := &MockDBProvider{
		DumpLocalFunc: func(ctx context.Context) (string, error) {
			return "INSERT INTO users VALUES ('local');", nil
		},
		BackupRemoteFunc: func(ctx context.Context) error {
			return nil
		},
		WriteRemoteFunc: func(ctx context.Context, sql string) error {
			if sql != "INSERT INTO users VALUES ('local');" {
				t.Errorf("Unexpected SQL: %s", sql)
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
