package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestWordPressFixtureImportsIntoMariaDB(t *testing.T) {
	if os.Getenv("DSYNC_INTEGRATION") != "1" {
		t.Skip("set DSYNC_INTEGRATION=1 to run Docker-backed MariaDB integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container := fmt.Sprintf("dsync-it-%d", time.Now().UnixNano())
	run := exec.CommandContext(ctx, "docker", "run", "--rm", "-d", "--name", container,
		"-e", "MARIADB_ROOT_PASSWORD=secret",
		"-e", "MARIADB_DATABASE=dsync_test",
		"mariadb:lts")
	if output, err := run.CombinedOutput(); err != nil {
		t.Fatalf("start mariadb container: %s: %v", output, err)
	}
	defer exec.Command("docker", "rm", "-f", container).Run()

	waitForMariaDB(ctx, t, container)

	data, err := os.ReadFile("testdata/wordpress-like.sql")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	transformed, err := transformStringForTest(string(data), goSerializedOptionsForTest(true, "guid"))
	if err != nil {
		t.Fatalf("transform fixture: %v", err)
	}

	importCmd := exec.CommandContext(ctx, "docker", "exec", "-i", container, "mariadb", "-h127.0.0.1", "-uroot", "-psecret", "dsync_test")
	importCmd.Stdin = strings.NewReader(transformed)
	if output, err := importCmd.CombinedOutput(); err != nil {
		t.Fatalf("import transformed fixture: %s: %v", output, err)
	}

	guid := queryMariaDB(ctx, t, container, "SELECT guid FROM wp_posts WHERE ID=1")
	if strings.TrimSpace(guid) != "https://example.com/?p=1" {
		t.Fatalf("guid was changed: %q", guid)
	}

	options := queryMariaDB(ctx, t, container, "SELECT option_value FROM wp_options ORDER BY option_id")
	for _, line := range strings.Split(strings.TrimSpace(options), "\n") {
		if isSerializedPHP(line) {
			if _, err := parsePHPSerialized(line); err != nil {
				t.Fatalf("invalid serialized option %q: %v", line, err)
			}
		}
	}
	if !strings.Contains(options, `s:17:"http://local.test";`) {
		t.Fatalf("serialized replacement missing from imported options: %s", options)
	}
}

func waitForMariaDB(ctx context.Context, t *testing.T, container string) {
	t.Helper()
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(2 * time.Minute)
	}
	var lastOutput []byte
	for time.Now().Before(deadline) {
		cmd := exec.CommandContext(ctx, "docker", "exec", container, "mariadb", "-h127.0.0.1", "-uroot", "-psecret", "dsync_test", "-e", "SELECT 1")
		output, err := cmd.CombinedOutput()
		if err == nil {
			return
		}
		lastOutput = output
		time.Sleep(time.Second)
	}
	t.Fatalf("mariadb did not become ready: %s", bytes.TrimSpace(lastOutput))
}

func queryMariaDB(ctx context.Context, t *testing.T, container string, query string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, "docker", "exec", container, "mariadb", "-h127.0.0.1", "-N", "-B", "-uroot", "-psecret", "dsync_test", "-e", query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("query %q: %s: %v", query, output, err)
	}
	return string(output)
}
