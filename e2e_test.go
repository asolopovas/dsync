package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestE2E runs the built dsync binary against a disposable Docker "remote"
// (MariaDB + sshd) and a disposable Docker Compose "local" MariaDB. Nothing
// outside the test temp directory, the containers it creates, or the repository
// testdata is read or written.
func TestE2E(t *testing.T) {
	if os.Getenv("DSYNC_INTEGRATION") != "1" {
		t.Skip("set DSYNC_INTEGRATION=1 to run the Docker-backed end-to-end test")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
	defer cancel()

	e := newE2E(ctx, t)

	t.Run("check-config", func(t *testing.T) {
		e.dsync(t, nil, "--check-config")
		env := e.readFile(t, filepath.Join(e.dir, ".env"))
		for _, want := range []string{"DSYNC_SSH_HOST=", "DSYNC_PORT=", "DSYNC_REMOTE_DB=", "DSYNC_SYNC_0_REMOTE=", "DSYNC_DBREPLACE_0_FROM=", "DSYNC_CONFIG="} {
			if !strings.Contains(env, want) {
				t.Fatalf(".env lacks %s:\n%s", want, env)
			}
		}
		if !strings.Contains(env, e.port) || !strings.Contains(env, e2eRemoteURL) {
			t.Fatalf(".env lacks migrated literals:\n%s", env)
		}
		config := e.readFile(t, filepath.Join(e.dir, "dsync-config.json"))
		if strings.Contains(config, e2eRemoteURL) || strings.Contains(config, "127.0.0.1") || !strings.Contains(config, "${DSYNC_SSH_HOST}") {
			t.Fatalf("config literals were not replaced by references:\n%s", config)
		}
	})

	t.Run("forward all skip uploads", func(t *testing.T) {
		e.dsync(t, nil, "-a", "--skip", "uploads")

		if got := e.readFile(t, filepath.Join(e.plugins, "hello/hello.php")); got != e2eHelloPHP {
			t.Fatalf("plugin file = %q", got)
		}
		if _, err := os.Stat(filepath.Join(e.plugins, "cache")); !os.IsNotExist(err) {
			t.Fatalf("excluded plugins/cache was synced: %v", err)
		}
		e.assertNoUploads(t)
		// Slash-escaped URL variants are replaced too.
		wantConfig := "window.cfg={\"home\":\"http://local.test\",\"escaped\":\"http:\\/\\/local.test\\/x\"};\n"
		if got := e.readFile(t, filepath.Join(e.plugins, "hello/config.js")); got != wantConfig {
			t.Fatalf("replace-enabled text file = %q, want %q", got, wantConfig)
		}

		e.assertSerializedValid(t, e.localQuery)
		home := e.localQuery(t, "SELECT option_value FROM wp_options WHERE option_name='home'")
		if home != serializedString(e2eLocalURL) {
			t.Fatalf("local home = %q", home)
		}
		upload := e.localQuery(t, "SELECT option_value FROM wp_options WHERE option_name='upload_path'")
		if upload != serializedString(e.root+"/wp-content/uploads") {
			t.Fatalf("local upload_path = %q", upload)
		}
		if got, want := e.localQuery(t, "SELECT option_value FROM wp_options WHERE option_name='multibyte'"), serializedString(e2eMultibyte+" "+e2eLocalURL); got != want {
			t.Fatalf("local multibyte option = %q, want %q", got, want)
		}
		if guid := e.localQuery(t, "SELECT guid FROM wp_posts WHERE ID=1"); guid != e2eRemoteURL+"/?p=1" {
			t.Fatalf("guid changed: %q", guid)
		}
		for _, id := range []string{"3", "4"} {
			query := "SELECT post_content FROM wp_posts WHERE ID=" + id
			if local, remote := e.localQuery(t, query), e.remoteQuery(t, query); local != remote {
				t.Fatalf("post %s differs: local %q remote %q", id, local, remote)
			}
		}

		calls := e.readFile(t, e.wpLog)
		want := "--path=" + e.root + " --skip-plugins --skip-themes --quiet cache flush\n"
		if calls != want {
			t.Fatalf("wp calls = %q, want %q", calls, want)
		}
	})

	t.Run("skip env then files", func(t *testing.T) {
		e.dsync(t, []string{"DSYNC_SKIP=uploads"}, "-f")
		e.assertNoUploads(t)
		e.dsync(t, nil, "-f")
		if got := e.readFile(t, filepath.Join(e.uploads, "2024/photo.txt")); got != "photo\n" {
			t.Fatalf("upload file = %q", got)
		}
	})

	t.Run("unknown skip fails", func(t *testing.T) {
		before := e.snapshot(t)
		out, err := e.runDsync(nil, "-a", "--skip", "nonexistent")
		if err == nil {
			t.Fatalf("dsync -a --skip nonexistent succeeded:\n%s", out)
		}
		if after := e.snapshot(t); after != before {
			t.Fatalf("failed dsync changed local state:\nbefore %s\nafter  %s\n%s", before, after, out)
		}
	})

	t.Run("reverse all skip uploads", func(t *testing.T) {
		changed := serializedArray("url", e2eLocalURL+"/changed")
		e.localExec(t, "UPDATE wp_options SET option_value='"+changed+"' WHERE option_name='nested'")
		newPHP := "<?php // changed locally\n"
		if err := os.WriteFile(filepath.Join(e.plugins, "hello/hello.php"), []byte(newPHP), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(e.uploads, "2024/local-only.txt"), []byte("local\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if e.remoteShell(t, "ls /root") != "" {
			t.Fatalf("remote login directory not empty before reverse sync")
		}

		e.dsync(t, nil, "-a", "-r", "--skip", "uploads")

		backups := strings.Fields(e.remoteShell(t, "ls /root/dsync-backup-*.sql"))
		if len(backups) != 1 {
			t.Fatalf("remote backups = %v", backups)
		}
		backup := e.remoteShell(t, "cat "+backups[0])
		if !strings.Contains(backup, "INSERT INTO `wp_options`") || !strings.Contains(backup, "'nested','"+strings.ReplaceAll(serializedArray("url", e2eRemoteURL), `"`, `\"`)+"'") || strings.Contains(backup, "/changed") {
			t.Fatalf("backup is not a dump of the pre-import state:\n%s", backup)
		}

		e.assertSerializedValid(t, e.remoteQuery)
		if got, want := e.remoteQuery(t, "SELECT option_value FROM wp_options WHERE option_name='nested'"), serializedArray("url", e2eRemoteURL+"/changed"); got != want {
			t.Fatalf("remote nested = %q, want %q", got, want)
		}
		if got := e.remoteQuery(t, "SELECT option_value FROM wp_options WHERE option_name='upload_path'"); got != serializedString(e2eRemoteRoot+"/wp-content/uploads") {
			t.Fatalf("remote upload_path = %q", got)
		}
		if got := e.remoteShell(t, "cat "+e2eRemoteRoot+"/wp-content/plugins/hello/hello.php"); got != newPHP {
			t.Fatalf("remote plugin file = %q", got)
		}
		if got := e.remoteShell(t, "ls "+e2eRemoteRoot+"/wp-content/uploads/2024"); strings.Contains(got, "local-only") {
			t.Fatalf("skipped uploads were pushed: %s", got)
		}
	})

	t.Run("db dump", func(t *testing.T) {
		e.dsync(t, nil, "-d", "--dump")
		dump := e.readFile(t, filepath.Join(e.dir, "db.sql"))
		if !strings.Contains(dump, "INSERT INTO `wp_options`") || !strings.Contains(dump, e2eLocalURL+"/changed") {
			t.Fatalf("db.sql is not the transformed dump:\n%s", dump)
		}
	})
}

const (
	e2eRemoteURL  = "https://example.com"
	e2eLocalURL   = "http://local.test"
	e2eRemoteRoot = "/srv/site"
	e2eDB         = "wp_e2e"
	e2eHelloPHP   = "<?php // hello\n"
	e2eConfigJS   = "window.cfg={\"home\":\"https://example.com\",\"escaped\":\"https:\\/\\/example.com\\/x\"};\n"
	// Multibyte text and a value with quotes and backslashes must survive both
	// directions byte for byte.
	e2eMultibyte = "Žąsų 日本語 🚀 café"
	e2eQuoted    = `It's "quoted" \ back\slash \\ end`
)

type e2e struct {
	ctx      context.Context
	dir      string
	root     string
	plugins  string
	uploads  string
	bin      string
	binDir   string
	wpLog    string
	compose  string
	remote   string
	port     string
	baseEnv  []string
	sqlQuote *strings.Replacer
}

func newE2E(ctx context.Context, t *testing.T) *e2e {
	t.Helper()
	dir := t.TempDir()
	id := strconv.FormatInt(time.Now().UnixNano(), 36)
	e := &e2e{
		ctx:      ctx,
		dir:      dir,
		root:     filepath.Join(dir, "site"),
		bin:      filepath.Join(dir, "dsync"),
		binDir:   filepath.Join(dir, "bin"),
		wpLog:    filepath.Join(dir, "wp-calls.log"),
		compose:  filepath.Join(dir, "docker-compose.yml"),
		remote:   "dsync-e2e-remote-" + id,
		sqlQuote: strings.NewReplacer(`\`, `\\`, `'`, `\'`),
	}
	e.plugins = filepath.Join(e.root, "wp-content/plugins")
	e.uploads = filepath.Join(e.root, "wp-content/uploads")
	image := "dsync-e2e-remote:" + id
	project := "dsync-e2e-local-" + id

	// Keep Docker's real client configuration while isolating HOME and ssh.
	realHome, _ := os.UserHomeDir()
	dockerConfig := os.Getenv("DOCKER_CONFIG")
	if dockerConfig == "" {
		dockerConfig = filepath.Join(realHome, ".docker")
	}
	realSSH, err := exec.LookPath("ssh")
	if err != nil {
		t.Fatalf("ssh: %v", err)
	}
	home := filepath.Join(dir, "home")
	for _, d := range []string{home, e.binDir, e.root, filepath.Join(e.root, "wp-content")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var env []string
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(key, "DSYNC_") || strings.HasPrefix(key, "SSH_") || key == "HOME" || key == "PATH" || key == "DOCKER_CONFIG" {
			continue
		}
		env = append(env, kv)
	}
	e.baseEnv = append(env,
		"HOME="+home,
		"DOCKER_CONFIG="+dockerConfig,
		"PATH="+e.binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"DSYNC_COMPOSE_FILE="+e.compose,
	)

	e.must(t, "", "go", "build", "-o", e.bin, ".")

	key := filepath.Join(dir, "id_ed25519")
	e.must(t, dir, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "dsync-e2e", "-f", key)
	sshConfig := filepath.Join(dir, "ssh_config")
	writeE2EFile(t, sshConfig, fmt.Sprintf("Host *\n  IdentityFile %s\n  IdentitiesOnly yes\n  IdentityAgent none\n  UserKnownHostsFile %s\n  StrictHostKeyChecking accept-new\n  BatchMode yes\n", key, filepath.Join(dir, "known_hosts")), 0o600)
	writeE2EFile(t, filepath.Join(e.binDir, "ssh"), fmt.Sprintf("#!/bin/sh\nexec %s -F %s \"$@\"\n", shellQuote(realSSH), shellQuote(sshConfig)), 0o755)
	writeE2EFile(t, filepath.Join(e.binDir, "wp"), fmt.Sprintf("#!/bin/sh\nexec echo \"$@\" >> %s\n", shellQuote(e.wpLog)), 0o755)
	writeE2EFile(t, filepath.Join(e.root, "wp-load.php"), "<?php\n", 0o644)

	// Remote container: removal runs even when setup fails part way.
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_ = exec.CommandContext(cleanup, "docker", "rm", "-f", "-v", e.remote).Run()
		_ = exec.CommandContext(cleanup, "docker", "rmi", "-f", image).Run()
		if _, err := os.Stat(e.compose); err == nil {
			_ = exec.CommandContext(cleanup, "docker", "compose", "-f", e.compose, "down", "-v", "--remove-orphans").Run()
		}
	})
	e.must(t, "", "docker", "build", "-q", "-t", image, "testdata/e2e-remote")
	e.must(t, dir, "docker", "run", "-d", "--name", e.remote, "-p", "127.0.0.1::22", "-e", "MARIADB_DATABASE="+e2eDB, image)
	hostPort := strings.TrimSpace(e.must(t, dir, "docker", "port", e.remote, "22/tcp"))
	e.port = hostPort[strings.LastIndex(hostPort, ":")+1:]
	e.must(t, dir, "docker", "cp", key+".pub", e.remote+":/root/.ssh/authorized_keys")
	e.must(t, dir, "docker", "exec", e.remote, "chown", "root:root", "/root/.ssh/authorized_keys")

	// Local Compose project: service "mariadb", root password "secret", no ports.
	writeE2EFile(t, e.compose, fmt.Sprintf("name: %s\nservices:\n  mariadb:\n    image: mariadb:lts\n    environment:\n      MARIADB_ROOT_PASSWORD: secret\n", project), 0o644)
	e.must(t, dir, "docker", "compose", "-f", e.compose, "up", "-d")

	e.waitFor(t, "remote mariadb", func() error {
		_, err := e.run(nil, dir, "docker", "exec", e.remote, "mariadb", "-h127.0.0.1", "-uroot", "-e", "SELECT 1", e2eDB)
		return err
	})
	e.waitFor(t, "local mariadb", func() error {
		_, err := e.run(nil, dir, "docker", "compose", "-f", e.compose, "exec", "-T", "mariadb", "mariadb", "-h127.0.0.1", "-uroot", "-psecret", "-e", "SELECT 1")
		return err
	})
	e.waitFor(t, "remote sshd", func() error {
		out, err := e.run(e.baseEnv, dir, filepath.Join(e.binDir, "ssh"), "-p", e.port, "root@127.0.0.1", "true")
		if err != nil {
			return fmt.Errorf("%w: %s", err, out)
		}
		return nil
	})

	e.seedRemote(t)
	e.writeConfig(t)
	return e
}

func (e *e2e) seedRemote(t *testing.T) {
	t.Helper()
	fixture, err := os.ReadFile("testdata/wordpress-like.sql")
	if err != nil {
		t.Fatal(err)
	}
	extra := fmt.Sprintf("INSERT INTO `wp_posts` (`ID`,`guid`,`post_content`) VALUES (3,'%[1]s/?p=3','%[2]s'),(4,'%[1]s/?p=4','%[3]s');\n"+
		"INSERT INTO `wp_options` (`option_id`,`option_name`,`option_value`) VALUES (4,'upload_path','%[4]s'),(5,'multibyte','%[5]s');\n",
		e2eRemoteURL, e.sqlQuote.Replace(e2eMultibyte), e.sqlQuote.Replace(e2eQuoted),
		serializedString(e2eRemoteRoot+"/wp-content/uploads"), e.sqlQuote.Replace(serializedString(e2eMultibyte+" "+e2eRemoteURL)))
	cmd := exec.CommandContext(e.ctx, "docker", "exec", "-i", e.remote, "mariadb", "-uroot", e2eDB)
	cmd.Stdin = strings.NewReader(string(fixture) + extra)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("seed remote db: %s: %v", out, err)
	}

	seed := filepath.Join(e.dir, "remote-seed")
	files := map[string]string{
		"wp-content/plugins/hello/hello.php":  e2eHelloPHP,
		"wp-content/plugins/hello/config.js":  e2eConfigJS,
		"wp-content/plugins/cache/cached.txt": "cached " + e2eRemoteURL + "\n",
		"wp-content/uploads/2024/photo.txt":   "photo\n",
	}
	for name, content := range files {
		path := filepath.Join(seed, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		writeE2EFile(t, path, content, 0o644)
	}
	e.remoteShell(t, "mkdir -p "+e2eRemoteRoot)
	e.must(t, e.dir, "docker", "cp", seed+"/.", e.remote+":"+e2eRemoteRoot)
	e.remoteShell(t, "chown -R root:root "+e2eRemoteRoot)
}

func (e *e2e) writeConfig(t *testing.T) {
	t.Helper()
	config := fmt.Sprintf(`{
  "sshHost": "root@127.0.0.1",
  "port": %q,
  "remote": { "host": "unused", "db": %q },
  "local": { "host": "unused", "db": %q },
  "dbReplace": [
    { "from": %q, "to": %q },
    { "from": %q, "to": %q }
  ],
  "sync": [
    { "remote": %q, "local": %q, "exclude": ["cache/"], "replace": true },
    { "remote": %q, "local": %q }
  ]
}
`, e.port, e2eDB, e2eDB, e2eRemoteURL, e2eLocalURL, e2eRemoteRoot, e.root,
		e2eRemoteRoot+"/wp-content/plugins", e.plugins, e2eRemoteRoot+"/wp-content/uploads", e.uploads)
	writeE2EFile(t, filepath.Join(e.dir, "dsync-config.json"), config, 0o600)
}

func (e *e2e) run(env []string, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(e.ctx, name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func (e *e2e) must(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	out, err := e.run(e.baseEnv, dir, name, args...)
	if err != nil {
		t.Fatalf("%s %s: %s: %v", name, strings.Join(args, " "), out, err)
	}
	return out
}

func (e *e2e) runDsync(extraEnv []string, args ...string) (string, error) {
	env := append(append([]string{}, e.baseEnv...), extraEnv...)
	return e.run(env, e.dir, e.bin, args...)
}

func (e *e2e) dsync(t *testing.T, extraEnv []string, args ...string) {
	t.Helper()
	if out, err := e.runDsync(extraEnv, args...); err != nil {
		t.Fatalf("dsync %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func (e *e2e) waitFor(t *testing.T, what string, ready func() error) {
	t.Helper()
	var err error
	for deadline := time.Now().Add(2 * time.Minute); time.Now().Before(deadline); time.Sleep(time.Second) {
		if err = ready(); err == nil {
			return
		}
	}
	t.Fatalf("%s did not become ready: %v", what, err)
}

func (e *e2e) remoteShell(t *testing.T, command string) string {
	t.Helper()
	return e.must(t, e.dir, "docker", "exec", e.remote, "sh", "-c", command)
}

// Queries return one HEX-encoded value decoded back to raw bytes, so
// comparisons are byte exact and independent of client escaping.
func (e *e2e) remoteQuery(t *testing.T, query string) string {
	t.Helper()
	return decodeHexRows(t, e.must(t, e.dir, "docker", "exec", e.remote, "mariadb", "-uroot", "-N", "-B", e2eDB, "-e", hexQuery(query)))
}

func (e *e2e) localQuery(t *testing.T, query string) string {
	t.Helper()
	return decodeHexRows(t, e.localExec(t, hexQuery(query)))
}

func (e *e2e) localExec(t *testing.T, sql string) string {
	t.Helper()
	return e.must(t, e.dir, "docker", "compose", "-f", e.compose, "exec", "-T", "mariadb", "mariadb", "-uroot", "-psecret", "-N", "-B", e2eDB, "-e", sql)
}

var selectColumn = regexp.MustCompile(`^SELECT (\w+) FROM`)

func hexQuery(query string) string {
	return selectColumn.ReplaceAllString(query, "SELECT HEX($1) FROM")
}

func decodeHexRows(t *testing.T, out string) string {
	t.Helper()
	var rows []string
	for _, line := range strings.Fields(out) {
		raw, err := hex.DecodeString(line)
		if err != nil {
			t.Fatalf("decode %q: %v", line, err)
		}
		rows = append(rows, string(raw))
	}
	return strings.Join(rows, "\n")
}

var serializedLength = regexp.MustCompile(`s:(\d+):"`)

// assertSerializedValid parses every serialized option and checks each
// declared s:N: length against the byte length of the string that follows.
func (e *e2e) assertSerializedValid(t *testing.T, query func(*testing.T, string) string) {
	t.Helper()
	for _, name := range []string{"home", "nested", "plugin", "upload_path", "multibyte"} {
		value := query(t, "SELECT option_value FROM wp_options WHERE option_name='"+name+"'")
		if _, err := parsePHPSerialized(value); err != nil {
			t.Fatalf("option %s is invalid serialized PHP %q: %v", name, value, err)
		}
		for _, match := range serializedLength.FindAllStringSubmatchIndex(value, -1) {
			declared, _ := strconv.Atoi(value[match[2]:match[3]])
			start := match[1]
			if start+declared+2 > len(value) || value[start+declared:start+declared+2] != `";` {
				t.Fatalf("option %s declares s:%d at byte %d but the string does not end there: %q", name, declared, start, value)
			}
		}
	}
}

func (e *e2e) assertNoUploads(t *testing.T) {
	t.Helper()
	entries, err := os.ReadDir(e.uploads)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) > 0 {
		t.Fatalf("uploads were synced despite skip: %v", entries)
	}
}

// snapshot fingerprints local files, the local database and the wp call log.
func (e *e2e) snapshot(t *testing.T) string {
	t.Helper()
	hash := sha256.New()
	err := filepath.WalkDir(e.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(hash, "%s %v %d\n", path, info.Mode(), info.ModTime().UnixNano())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			hash.Write(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	hash.Write([]byte(e.localExec(t, "SELECT option_id, HEX(option_value) FROM wp_options ORDER BY option_id; SELECT ID, HEX(guid), HEX(post_content) FROM wp_posts ORDER BY ID")))
	wp, _ := os.ReadFile(e.wpLog)
	hash.Write(wp)
	return hex.EncodeToString(hash.Sum(nil))
}

func (e *e2e) readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeE2EFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func serializedString(value string) string {
	return fmt.Sprintf("s:%d:\"%s\";", len(value), value)
}

func serializedArray(key, value string) string {
	return fmt.Sprintf("a:1:{%s%s}", serializedString(key), serializedString(value))
}
