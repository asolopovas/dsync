package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pterm/pterm"
)

type DBDump struct {
	Reader io.ReadCloser
	Wait   func() error
	cancel context.CancelFunc
}

type DBProvider interface {
	DumpRemote(ctx context.Context) (*DBDump, error)
	DumpLocal(ctx context.Context) (*DBDump, error)
	WriteRemote(ctx context.Context, sql io.Reader) error
	WriteLocal(ctx context.Context, sql io.Reader) error
	BackupRemote(ctx context.Context) error
	PreflightLocalCache(ctx context.Context, wordpressRoots []string) error
	FlushLocalCache(ctx context.Context, wordpressRoots []string) error
}

type RealDBProvider struct {
	cfg          *Config
	wpExecutable string
}

func NewRealDBProvider(cfg *Config) *RealDBProvider {
	return &RealDBProvider{cfg: cfg}
}

func SyncDB(ctx context.Context, provider DBProvider, cfg *Config, dumpDB bool, reverse bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	warnUnsafeRawReplacement(cfg)
	if reverse {
		return syncDBReverse(ctx, provider, cfg, dumpDB)
	}

	pterm.DefaultSection.Println("Syncing Database (remote to local)")
	wordpressRoots, err := localWordPressRoots(cfg)
	if err != nil {
		return fmt.Errorf("failed to discover local WordPress roots: %w", err)
	}

	stageCount := 3
	if len(wordpressRoots) > 0 {
		stageCount = 4
		spinner := startSpinner("Preflight: checking local WordPress cache invalidation...")
		if err := provider.PreflightLocalCache(ctx, wordpressRoots); err != nil {
			spinner.Fail(fmt.Sprintf("Preflight failed before local database import: %v", err))
			return fmt.Errorf("WordPress cache preflight failed before local database import: %w", err)
		}
		spinner.Success(fmt.Sprintf("Preflight complete: WordPress cache invalidation ready for %d site(s)", len(wordpressRoots)))
	}

	spinner := startSpinner(fmt.Sprintf("Stage 1/%d: starting remote database dump '%s'...", stageCount, cfg.Remote.DB))
	dump, err := provider.DumpRemote(ctx)
	if err != nil {
		spinner.Fail(fmt.Sprintf("Stage 1/%d failed: remote database dump: %v", stageCount, err))
		return fmt.Errorf("failed to dump remote db: %w", err)
	}
	dump.cancel = cancel
	spinner.Success(fmt.Sprintf("Stage 1/%d complete: remote dump stream started for '%s'", stageCount, cfg.Remote.DB))

	progress := &dbStreamProgress{}
	label := fmt.Sprintf("DB 2/%d transform (%s) + 3/%d import local '%s'", stageCount, ReplacementOptionsFromConfig(cfg, cfg.DBReplace).Engine, stageCount, cfg.Local.DB)
	spinner = startSpinner(label + "...")
	stopProgress := startDBProgress(ctx, spinner, label, progress)
	if err := writeTransformedDump(ctx, dump, cfg, cfg.DBReplace, dumpDB, "db.sql", provider.WriteLocal, progress); err != nil {
		stopProgress()
		spinner.Fail(fmt.Sprintf("Stage 2/%d + 3/%d failed: local database import: %v", stageCount, stageCount, err))
		return fmt.Errorf("failed to write to local db: %w", err)
	}
	stopProgress()
	spinner.Success(fmt.Sprintf("Stage 3/%d complete: wrote %s to local database '%s'", stageCount, formatBytes(progress.outputBytes.Load()), cfg.Local.DB))

	if len(wordpressRoots) > 0 {
		spinner = startSpinner(fmt.Sprintf("Stage 4/4: invalidating WordPress object cache for %d site(s)...", len(wordpressRoots)))
		if err := provider.FlushLocalCache(ctx, wordpressRoots); err != nil {
			spinner.Fail(fmt.Sprintf("Stage 4/4 failed: database imported successfully, but WordPress cache invalidation failed: %v", err))
			return fmt.Errorf("local database '%s' imported successfully, but WordPress cache invalidation failed: %w", cfg.Local.DB, err)
		}
		spinner.Success(fmt.Sprintf("Stage 4/4 complete: invalidated WordPress object cache for %d site(s)", len(wordpressRoots)))
	}

	return nil
}

func localWordPressRoots(cfg *Config) ([]string, error) {
	roots := make([]string, 0, len(cfg.Sync))
	seen := make(map[string]struct{}, len(cfg.Sync))

	for _, syncPath := range cfg.Sync {
		path := filepath.Clean(syncPath.Local)
		for {
			if strings.EqualFold(filepath.Base(path), "wp-content") {
				root, err := filepath.Abs(filepath.Dir(path))
				if err != nil {
					return nil, fmt.Errorf("resolve WordPress root from %q: %w", syncPath.Local, err)
				}
				root = filepath.Clean(root)
				if _, exists := seen[root]; !exists {
					seen[root] = struct{}{}
					roots = append(roots, root)
				}
				break
			}

			parent := filepath.Dir(path)
			if parent == path {
				break
			}
			path = parent
		}
	}

	return roots, nil
}

func syncDBReverse(ctx context.Context, provider DBProvider, cfg *Config, dumpDB bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	pterm.DefaultSection.Println("Syncing Database (local to remote)")

	spinner := startSpinner(fmt.Sprintf("Stage 1/4: starting local database dump '%s'...", cfg.Local.DB))
	dump, err := provider.DumpLocal(ctx)
	if err != nil {
		spinner.Fail(fmt.Sprintf("Stage 1/4 failed: local database dump: %v", err))
		return fmt.Errorf("failed to dump local db: %w", err)
	}
	dump.cancel = cancel
	spinner.Success(fmt.Sprintf("Stage 1/4 complete: local dump stream started for '%s'", cfg.Local.DB))

	var reversedReplacements []DBReplace
	for _, r := range slices.Backward(cfg.DBReplace) {
		reversedReplacements = append(reversedReplacements, DBReplace{From: r.To, To: r.From})
	}

	spinner = startSpinner("Stage 2/4: backing up remote database before import...")
	if err := provider.BackupRemote(ctx); err != nil {
		cancel()
		_ = dump.Reader.Close()
		err = errors.Join(err, dump.Wait())
		spinner.Fail(fmt.Sprintf("Stage 2/4 failed: remote database backup: %v", err))
		return fmt.Errorf("failed to backup remote db: %w", err)
	}
	spinner.Success("Stage 2/4 complete: remote database backup created")

	progress := &dbStreamProgress{}
	label := fmt.Sprintf("DB 3/4 reverse transform (%s) + 4/4 import remote '%s'", ReplacementOptionsFromConfig(cfg, reversedReplacements).Engine, cfg.Remote.DB)
	spinner = startSpinner(label + "...")
	stopProgress := startDBProgress(ctx, spinner, label, progress)
	if err := writeTransformedDump(ctx, dump, cfg, reversedReplacements, dumpDB, "db_reverse.sql", provider.WriteRemote, progress); err != nil {
		stopProgress()
		spinner.Fail(fmt.Sprintf("Stage 3/4 + 4/4 failed: remote database import: %v", err))
		return fmt.Errorf("failed to write to remote db: %w", err)
	}
	stopProgress()
	spinner.Success(fmt.Sprintf("Stage 4/4 complete: wrote %s to remote database '%s'", formatBytes(progress.outputBytes.Load()), cfg.Remote.DB))

	return nil
}

// writeTransformedDump owns the dump, transformer, and optional file. Every
// exit closes the stream and reaps the process exactly once.
func writeTransformedDump(ctx context.Context, dump *DBDump, cfg *Config, replacements []DBReplace, dumpDB bool, dumpPath string, writeDB func(context.Context, io.Reader) error, progress *dbStreamProgress) (result error) {
	parent := ctx
	ctx, cancel := context.WithCancel(ctx)
	closeSource := sync.OnceFunc(func() { _ = dump.Reader.Close() })
	defer cancel()
	defer func() {
		if result != nil {
			cancel()
			if dump.cancel != nil {
				dump.cancel()
			}
		}
		closeSource()
		result = errors.Join(result, dump.Wait())
	}()
	var dumpFile *os.File
	if dumpDB {
		file, err := createPrivateDump(dumpPath)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", dumpPath, err)
		}
		dumpFile = file
		defer func() { result = errors.Join(result, dumpFile.Close()) }()
	}
	var source io.Reader = dump.Reader
	if progress != nil {
		source = &countingReader{reader: source, counter: &progress.sourceBytes}
	}
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	stopped := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		closeSource()
		_ = reader.CloseWithError(ctx.Err())
		_ = writer.CloseWithError(ctx.Err())
		close(stopped)
	})
	defer func() {
		if !stop() {
			<-stopped
		}
	}()
	go func() {
		err := TransformSQLDump(&contextReader{ctx: ctx, reader: source}, writer, ReplacementOptionsFromConfig(cfg, replacements))
		_ = writer.CloseWithError(err)
		if err != nil {
			cancel()
		}
		done <- err
	}()
	var input io.Reader = reader
	if progress != nil {
		input = &countingReader{reader: input, counter: &progress.outputBytes}
	}
	if dumpFile != nil {
		input = io.TeeReader(input, dumpFile)
	}
	writeErr := writeDB(ctx, input)
	// Also handle consumers that return successfully without draining the stream.
	_ = reader.Close()
	closeSource()
	if writeErr != nil {
		cancel()
		closeSource()
	}
	transformErr := <-done
	if writeErr != nil && (errors.Is(transformErr, io.ErrClosedPipe) || errors.Is(transformErr, context.Canceled)) {
		transformErr = nil
	}
	return errors.Join(writeErr, transformErr, parent.Err())
}

// Use an exclusive temporary inode then rename, so an existing symlink is never
// followed and existing dumps cannot retain public permissions or hard links.
func createPrivateDump(path string) (*os.File, error) {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("dump destination is not a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".dsync-dump-*")
	if err != nil {
		return nil, err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return nil, err
	}
	return file, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func warnUnsafeRawReplacement(cfg *Config) {
	if strings.EqualFold(strings.TrimSpace(cfg.DBReplaceEngine), DBReplaceEngineRaw) && isWordPressLikeConfig(cfg) {
		pterm.Warning.Println("dbReplaceEngine raw is unsafe for WordPress serialized data; use go-serialized unless you accept corruption risk")
	}
}

func isWordPressLikeConfig(cfg *Config) bool {
	for _, path := range cfg.Sync {
		if strings.Contains(strings.ToLower(path.Remote), "wp-content") || strings.Contains(strings.ToLower(path.Local), "wp-content") {
			return true
		}
	}
	return false
}

type dbStreamProgress struct {
	sourceBytes atomic.Int64
	outputBytes atomic.Int64
}

type countingReader struct {
	reader  io.Reader
	counter *atomic.Int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.counter.Add(int64(n))
	}
	return n, err
}

func startDBProgress(ctx context.Context, spinner *pterm.SpinnerPrinter, label string, progress *dbStreamProgress) func() {
	done := make(chan struct{})
	stopped := make(chan struct{})
	started := time.Now()
	lastProgressAt := started
	lastTotal := int64(0)

	go func() {
		defer close(stopped)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sourceBytes := progress.sourceBytes.Load()
				outputBytes := progress.outputBytes.Load()
				total := sourceBytes + outputBytes
				if total != lastTotal {
					lastTotal = total
					lastProgressAt = time.Now()
				}
				spinner.UpdateText(dbProgressText(label, sourceBytes, outputBytes, time.Since(started), time.Since(lastProgressAt)))
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return func() {
		close(done)
		<-stopped
	}
}

func dbProgressText(label string, sourceBytes, outputBytes int64, elapsed, idle time.Duration) string {
	message := fmt.Sprintf("%s (read %s, sent %s, elapsed %s)", label, formatBytes(sourceBytes), formatBytes(outputBytes), elapsed.Round(time.Second))
	if idle >= 10*time.Second {
		message += fmt.Sprintf(" — idle %s", idle.Round(time.Second))
	}
	return message
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	value := float64(bytes)
	for _, suffix := range []string{"KiB", "MiB", "GiB", "TiB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f PiB", value/unit)
}

func (p *RealDBProvider) DumpRemote(ctx context.Context) (*DBDump, error) {
	args := sshArgs(p.cfg, shellCommand(append(append([]string{"mysqldump", "-uroot"}, mysqlDumpFlags()...), "--", p.cfg.Remote.DB)...))

	cmd := exec.CommandContext(ctx, "ssh", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to open ssh stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		return nil, fmt.Errorf("failed to start ssh dump command: %s: %w", stderr.String(), err)
	}

	return &DBDump{
		Reader: stdout,
		Wait: func() error {
			if err := cmd.Wait(); err != nil {
				return fmt.Errorf("ssh dump command failed: %s: %w", stderr.String(), err)
			}
			return nil
		},
	}, nil
}

func (p *RealDBProvider) DumpLocal(ctx context.Context) (*DBDump, error) {
	args := composeArgs(getComposeFilePath(), "sh", "-c", localDumpCommand(p.cfg.Local.DB))

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to open docker stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		return nil, fmt.Errorf("failed to start docker dump command: %s: %w", stderr.String(), err)
	}

	return &DBDump{
		Reader: stdout,
		Wait: func() error {
			if err := cmd.Wait(); err != nil {
				return fmt.Errorf("docker dump command failed: %s: %w", stderr.String(), err)
			}
			return nil
		},
	}, nil
}

func (p *RealDBProvider) WriteRemote(ctx context.Context, sqlDump io.Reader) error {
	args := sshArgs(p.cfg, shellCommand("mysql", "-uroot", "--", p.cfg.Remote.DB))

	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdin = sqlDump
	cmd.WaitDelay = time.Second
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh command failed: %s: %w", string(output), err)
	}

	return nil
}

func (p *RealDBProvider) WriteLocal(ctx context.Context, sqlDump io.Reader) error {
	composeFile := getComposeFilePath()

	if err := ensureUserAndDB(ctx, p.cfg.Local.DB, composeFile); err != nil {
		return err
	}

	args := composeArgs(composeFile, "mariadb", "-uroot", "-psecret", "--", p.cfg.Local.DB)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = sqlDump
	cmd.WaitDelay = time.Second
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker command failed: %s: %w", string(output), err)
	}

	return nil
}

func (p *RealDBProvider) BackupRemote(ctx context.Context) error {
	args := sshArgs(p.cfg, remoteBackupCommand(p.cfg.Remote.DB))

	cmd := exec.CommandContext(ctx, "ssh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh backup command failed: %s: %w", string(output), err)
	}
	return nil
}

func (p *RealDBProvider) PreflightLocalCache(ctx context.Context, wordpressRoots []string) error {
	for _, root := range wordpressRoots {
		if err := ctx.Err(); err != nil {
			return err
		}

		info, err := os.Stat(root)
		if err != nil {
			return fmt.Errorf("local WordPress root %q is unavailable: %w", root, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("local WordPress root %q is not a directory", root)
		}

		wpLoadPath := filepath.Join(root, "wp-load.php")
		info, err = os.Stat(wpLoadPath)
		if err != nil {
			return fmt.Errorf("local WordPress root %q is invalid: %s is unavailable: %w", root, wpLoadPath, err)
		}
		if info.IsDir() {
			return fmt.Errorf("local WordPress root %q is invalid: %s is not a file", root, wpLoadPath)
		}
	}

	wpExecutable, err := exec.LookPath("wp")
	if err != nil {
		return fmt.Errorf("WP-CLI executable 'wp' is unavailable: %w", err)
	}
	p.wpExecutable = wpExecutable
	return nil
}

func (p *RealDBProvider) FlushLocalCache(ctx context.Context, wordpressRoots []string) error {
	if p.wpExecutable == "" {
		return fmt.Errorf("WP-CLI cache flush was not preflighted")
	}

	for _, root := range wordpressRoots {
		args := []string{
			"--path=" + root,
			"--skip-plugins",
			"--skip-themes",
			"--quiet",
			"cache", "flush",
		}
		cmd := exec.CommandContext(ctx, p.wpExecutable, args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("WP-CLI cache flush failed for %q: %s: %w", root, strings.TrimSpace(string(output)), err)
		}
	}

	return nil
}

func ensureUserAndDB(ctx context.Context, dbName, composeFile string) error {
	args := composeArgs(composeFile, "mariadb", "-uroot", "-psecret", "-e", createUserAndDBQuery(dbName))

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create user/db: %s: %w", string(output), err)
	}
	return nil
}

func getComposeFilePath() string {
	if path := os.Getenv("DSYNC_COMPOSE_FILE"); path != "" {
		return path
	}
	return os.Getenv("HOME") + "/www/dev/docker-compose.yml"
}

func mysqlDumpFlags() []string {
	return []string{
		"--single-transaction",
		"--quick",
		"--hex-blob",
		"--complete-insert",
		"--extended-insert",
		"--default-character-set=utf8mb4",
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func ApplyDBReplacements(sql string, replacements []DBReplace) string {
	return applyStringReplacements(sql, replacements)
}

type compiledReplacements []DBReplace

func compileReplacements(replacements []DBReplace) compiledReplacements {
	compiled := make(compiledReplacements, 0, len(replacements)*6)
	for _, item := range replacements {
		if item.From == "" {
			continue
		} // Validation rejects empty effective sources.
		for _, slash := range []string{`\/`, `\\/`, `\\\/`, `\\\\/`, `\\\\\/`} {
			from := strings.ReplaceAll(item.From, "/", slash)
			if from != item.From {
				compiled = append(compiled, DBReplace{From: from, To: strings.ReplaceAll(item.To, "/", slash)})
			}
		}
		compiled = append(compiled, item)
	}
	return compiled
}
func (compiled compiledReplacements) apply(value string) string {
	for _, item := range compiled {
		value = strings.ReplaceAll(value, item.From, item.To)
	}
	return value
}
func applyStringReplacements(value string, replacements []DBReplace) string {
	return compileReplacements(replacements).apply(value)
}

func sshArgs(cfg *Config, command string) []string {
	return []string{"-p", cfg.Port, "--", cfg.SSHHost, command}
}
func shellCommand(args ...string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = shellQuote(arg)
	}
	return strings.Join(quoted, " ")
}
func composeArgs(file string, args ...string) []string {
	return append([]string{"compose", "-f", file, "exec", "-T", "mariadb"}, args...)
}
func localDumpCommand(db string) string {
	args := append([]string{"-uroot", "-psecret"}, mysqlDumpFlags()...)
	args = append(args, "--", db)
	return "if command -v mariadb-dump >/dev/null 2>&1; then " + shellCommand(append([]string{"mariadb-dump"}, args...)...) + "; else " + shellCommand(append([]string{"mysqldump"}, args...)...) + "; fi"
}
func remoteBackupCommand(db string) string {
	args := append([]string{"mysqldump", "-uroot"}, mysqlDumpFlags()...)
	args = append(args, "--", db)
	// mktemp creates a private, collision-safe file in the remote working directory.
	// Keep partial backups on failure for diagnosis; never proceed with import.
	return "umask 077; backup=$(mktemp ./dsync-backup-XXXXXXXXXX.sql) && " + shellCommand(args...) + " > \"$backup\""
}
func sqlIdentifier(value string) string { return "`" + strings.ReplaceAll(value, "`", "``") + "`" }
func sqlAccount(value string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), "'", "''") + "'"
}
func createUserAndDBQuery(db string) string {
	return fmt.Sprintf("CREATE USER IF NOT EXISTS %s@'%%' IDENTIFIED BY 'secret'; CREATE DATABASE IF NOT EXISTS %s; GRANT ALL PRIVILEGES ON %s.* TO %s@'%%';", sqlAccount(db), sqlIdentifier(db), sqlIdentifier(db), sqlAccount(db))
}
