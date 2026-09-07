package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestConfigValidationOperations(t *testing.T) {
	valid := func() *Config {
		return &Config{SSHHost: "user@example.com", Port: "22", Local: HostSettings{DB: "-local ` '"}, Remote: HostSettings{DB: "remote"}, Sync: []SyncPath{{Local: "-local", Remote: "/remote"}}}
	}
	for _, tc := range []struct {
		name                        string
		edit                        func(*Config)
		files, db, reverse, wantErr bool
	}{
		{"files need no DB", func(c *Config) { c.Local.DB = ""; c.Remote.DB = "" }, true, false, false, false},
		{"DB needs no paths", func(c *Config) { c.Sync = nil }, false, true, false, false},
		{"missing paths", func(c *Config) { c.Sync = nil }, true, false, false, true},
		{"missing DB", func(c *Config) { c.Local.DB = "" }, false, true, false, true},
		{"host option", func(c *Config) { c.SSHHost = "-oProxyCommand=x" }, true, false, false, true},
		{"host shell", func(c *Config) { c.SSHHost = "user@host;id" }, true, false, false, true},
		{"port shell", func(c *Config) { c.Port = "22 -oProxyCommand=x" }, true, false, false, true},
		{"port range", func(c *Config) { c.Port = "65536" }, true, false, false, true},
		{"unknown engine", func(c *Config) { c.DBReplaceEngine = "typo" }, false, true, false, true},
		{"empty source", func(c *Config) { c.DBReplace = []DBReplace{{To: "x"}} }, false, true, false, true},
		{"empty reverse source", func(c *Config) { c.DBReplace = []DBReplace{{From: "x"}} }, false, true, true, true},
		{"forward deletion", func(c *Config) { c.DBReplace = []DBReplace{{From: "x"}} }, false, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := valid()
			tc.edit(c)
			err := c.Validate(tc.files, tc.db, tc.reverse)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate=%v", err)
			}
		})
	}
}

func TestConfigPrivateExclusiveAndUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := GenerateConfig(path); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode %v", info.Mode())
	}
	before := readTestFile(t, path)
	if err := GenerateConfig(path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("second generation=%v", err)
	}
	if readTestFile(t, path) != before {
		t.Fatal("overwritten")
	}
	writeTestFile(t, path, `{"sshHost":"host","port":"22","future":{"enabled":true}}`)
	if _, err := LoadConfig(path); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if err := GenerateConfig(link); !errors.Is(err, os.ErrExist) {
		t.Fatalf("symlink generation=%v", err)
	}
}

func TestRootRejectsArgumentsBeforeConfig(t *testing.T) {
	for _, args := range [][]string{{"stray"}, {"-f", "--dump"}, {"--dump"}, {"completion", "stray"}} {
		cmd := newRootCmd()
		cmd.SetArgs(args)
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		if err := cmd.ExecuteContext(t.Context()); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestRootFailedRsyncStopsDispatch(t *testing.T) {
	bin, dir := t.TempDir(), t.TempDir()
	calls := filepath.Join(dir, "calls")
	t.Setenv("DSYNC_TEST_CALLS", calls)
	writeTestFile(t, filepath.Join(bin, "rsync"), "#!/bin/sh\necho rsync >> \"$DSYNC_TEST_CALLS\"\necho rsync-denied >&2\nexit 7\n")
	if err := os.Chmod(filepath.Join(bin, "rsync"), 0755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(bin, "ssh"), "#!/bin/sh\necho ssh >> \"$DSYNC_TEST_CALLS\"\nexit 9\n")
	if err := os.Chmod(filepath.Join(bin, "ssh"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	target := filepath.Join(dir, "x.css")
	writeTestFile(t, target, "remote")
	cfg := Config{SSHHost: "host", Port: "22", Local: HostSettings{DB: "local"}, Remote: HostSettings{DB: "remote"}, DBReplace: []DBReplace{{From: "remote", To: "local"}}, Sync: []SyncPath{{Local: dir, Remote: "/remote", Replace: true}, {Local: dir, Remote: "/other"}}}
	data, _ := json.Marshal(cfg)
	config := filepath.Join(dir, "config.json")
	writeTestFile(t, config, string(data))
	for _, flags := range [][]string{{"-a"}, {"-f", "-d"}, {"-a", "-f", "-d"}} {
		writeTestFile(t, calls, "")
		cmd := newRootCmd()
		cmd.SetArgs(append(flags, "-c", config))
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		err := cmd.ExecuteContext(t.Context())
		if err == nil || !strings.Contains(err.Error(), "rsync-denied") {
			t.Fatalf("dispatch=%v", err)
		}
		if got := readTestFile(t, calls); got != "rsync\n" {
			t.Fatalf("calls=%q", got)
		}
		if readTestFile(t, target) != "remote" {
			t.Fatal("replacement after failure")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	cmd := newRootCmd()
	cmd.SetArgs([]string{"-a", "-c", config})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
}

func TestCommandBuildersShellRoundTrip(t *testing.T) {
	args := []string{"space here", "single'quote", `double"quote`, ";$(touch never)", "-option", "`quoted`", "back\\slash"}
	cmd := exec.CommandContext(t.Context(), "sh", "-c", "printf '%s\\000' "+shellCommand(args...))
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00"); !reflect.DeepEqual(got, args) {
		t.Fatalf("args=%q", got)
	}
	cfg := &Config{SSHHost: "user@host", Port: "2222"}
	colonArgs := rsyncArgs(cfg, SyncPath{}, ":relative/", "./local/", false)
	if colonArgs[len(colonArgs)-2] != "user@host:./:relative/" {
		t.Fatalf("daemon endpoint injection: %q", colonArgs)
	}
	if got := sshArgs(cfg, "command"); !reflect.DeepEqual(got, []string{"-p", "2222", "--", "user@host", "command"}) {
		t.Fatal(got)
	}
	for _, reverse := range []bool{false, true} {
		got := rsyncArgs(cfg, SyncPath{Exclude: []string{"-x", "space here"}}, "/remote ' ;/", "-local:/", reverse)
		want := []string{"user@host:/remote ' ;/", "./-local:/"}
		if reverse {
			want[0], want[1] = want[1], want[0]
		}
		if !reflect.DeepEqual(got[len(got)-2:], want) || got[len(got)-3] != "--" {
			t.Fatalf("rsync=%q", got)
		}
	}
	db := "-name`'\\;"
	if got := sqlIdentifier(db); got != "`-name``'\\;`" {
		t.Fatal(got)
	}
	if got := sqlAccount(db); got != "'-name`''\\\\;'" {
		t.Fatal(got)
	}
	if !strings.Contains(localDumpCommand(db), "'--' "+shellQuote(db)) {
		t.Fatal("missing option terminator")
	}
	if got := composeArgs("-file with space", "mariadb", "--", db); got[2] != "-file with space" || got[len(got)-1] != db {
		t.Fatal(got)
	}
}

func TestPrivateDumps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dump.sql")
	writeTestFile(t, path, "old")
	file, err := createPrivateDump(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("new"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 || readTestFile(t, path) != "new" {
		t.Fatal("dump not private")
	}
	link := filepath.Join(t.TempDir(), "dump.sql")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if file, err := createPrivateDump(link); err == nil {
		file.Close()
		t.Fatal("accepted symlink")
	}
	if readTestFile(t, path) != "new" {
		t.Fatal("symlink target changed")
	}
}

func TestRemoteBackupIsPrivateAndCollisionSafe(t *testing.T) {
	dir, bin := t.TempDir(), t.TempDir()
	script := filepath.Join(bin, "mysqldump")
	writeTestFile(t, script, "#!/bin/sh\nprintf 'synthetic dump'\n")
	if err := os.Chmod(script, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	for range 2 {
		cmd := exec.CommandContext(t.Context(), "sh", "-c", remoteBackupCommand("-db' ;"))
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v", out, err)
		}
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 2 {
		t.Fatalf("backups=%d", len(files))
	}
	for _, entry := range files {
		info, _ := entry.Info()
		if info.Mode().Perm() != 0600 {
			t.Fatal(info.Mode())
		}
	}
}

type failingReadCloser struct {
	err    error
	closed bool
}

func (r *failingReadCloser) Read([]byte) (int, error) { return 0, r.err }
func (r *failingReadCloser) Close() error             { r.closed = true; return nil }

func TestPipelineOwnsEveryExit(t *testing.T) {
	primary, waitFailure := errors.New("primary"), errors.New("dump stderr diagnostic")
	for _, name := range []string{"success", "write", "read", "transform", "create", "wait"} {
		t.Run(name, func(t *testing.T) {
			reader := newCloseSignalReadCloser("SELECT 1;")
			var source io.ReadCloser = reader
			if name == "read" {
				source = &failingReadCloser{err: primary}
			}
			if name == "transform" {
				source = newCloseSignalReadCloser("INSERT INTO t VALUES ('x';")
			}
			waits := 0
			dump := &DBDump{Reader: source, Wait: func() error {
				waits++
				if name == "wait" || name == "write" {
					return waitFailure
				}
				return nil
			}}
			cfg := &Config{DBReplaceEngine: DBReplaceEngineGoSerialized}
			wrote := false
			err := writeTransformedDump(t.Context(), dump, cfg, nil, name == "create", filepath.Join(t.TempDir(), "absent", "dump.sql"), func(_ context.Context, r io.Reader) error {
				wrote = true
				if name == "write" {
					return primary
				}
				_, err := io.Copy(io.Discard, r)
				return err
			}, nil)
			if waits != 1 {
				t.Fatalf("Wait calls=%d", waits)
			}
			if name == "success" && err != nil {
				t.Fatal(err)
			}
			if name != "success" && err == nil {
				t.Fatal("expected error")
			}
			if (name == "read" || name == "write") && !errors.Is(err, primary) {
				t.Fatalf("lost primary: %v", err)
			}
			if (name == "wait" || name == "write") && !errors.Is(err, waitFailure) {
				t.Fatalf("lost diagnostics: %v", err)
			}
			if name == "create" && wrote {
				t.Fatal("import after file failure")
			}
		})
	}
}

func TestPipelineCancellationUnblocksRead(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	reader, writer := io.Pipe()
	defer writer.Close()
	entered := make(chan struct{})
	result := make(chan error, 1)
	waits := 0
	go func() {
		result <- writeTransformedDump(ctx, &DBDump{Reader: reader, Wait: func() error { waits++; return nil }}, &Config{}, nil, false, "", func(_ context.Context, r io.Reader) error {
			close(entered)
			_, err := io.Copy(io.Discard, r)
			return err
		}, nil)
	}()
	<-entered
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if waits != 1 {
			t.Fatal(waits)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("pipeline leaked")
	}
}

func TestBackupFailureReapsDump(t *testing.T) {
	sentinel := errors.New("backup failed")
	waits := 0
	reader := newCloseSignalReadCloser("sql")
	provider := &mockDBProvider{DumpLocalFunc: func(context.Context) (*DBDump, error) {
		return &DBDump{Reader: reader, Wait: func() error { waits++; return nil }}, nil
	}, BackupRemoteFunc: func(context.Context) error { return sentinel }}
	err := SyncDB(t.Context(), provider, dbSyncConfig(), false, true)
	if !errors.Is(err, sentinel) || waits != 1 {
		t.Fatalf("err=%v waits=%d", err, waits)
	}
	select {
	case <-reader.closed:
	default:
		t.Fatal("reader not closed")
	}
	requireCalls(t, provider.Calls, []string{"DumpLocal", "BackupRemote"})
}

func TestSourceErrorSurvivesSerializedTransform(t *testing.T) {
	sentinel := errors.New("source read failure")
	for _, engine := range []string{DBReplaceEngineNone, DBReplaceEngineRaw, DBReplaceEngineGoSerialized} {
		input := io.MultiReader(strings.NewReader("INSERT INTO t VALUES ('x')"), &failingReadCloser{err: sentinel})
		var output bytes.Buffer
		err := TransformSQLDump(input, &output, ReplacementOptions{Engine: engine})
		if !errors.Is(err, sentinel) {
			t.Fatalf("%s: %v", engine, err)
		}
	}
}

func TestPipelineWriteFailureUnblocksSourceAndPreservesDiagnostics(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()
	sentinel := errors.New("import denied")
	done := make(chan error, 1)
	go func() {
		done <- writeTransformedDump(t.Context(), &DBDump{Reader: reader, Wait: func() error { return nil }}, &Config{}, nil, false, "", func(context.Context, io.Reader) error { return sentinel }, nil)
	}()
	select {
	case err := <-done:
		if !errors.Is(err, sentinel) {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("write failure left source blocked")
	}
}
