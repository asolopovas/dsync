package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pterm/pterm"
)

type DBDump struct {
	Reader io.ReadCloser
	Wait   func() error
}

type DBProvider interface {
	DumpRemote(ctx context.Context) (*DBDump, error)
	DumpLocal(ctx context.Context) (*DBDump, error)
	WriteRemote(ctx context.Context, sql io.Reader) error
	WriteLocal(ctx context.Context, sql io.Reader) error
	BackupRemote(ctx context.Context) error
}

type RealDBProvider struct {
	cfg *Config
}

func NewRealDBProvider(cfg *Config) *RealDBProvider {
	return &RealDBProvider{cfg: cfg}
}

func SyncDB(ctx context.Context, provider DBProvider, cfg *Config, dumpDB bool, reverse bool) error {
	warnUnsafeRawReplacement(cfg)
	if reverse {
		return syncDBReverse(ctx, provider, cfg, dumpDB)
	}

	pterm.DefaultSection.Println("Syncing Database (remote to local)")

	spinner := startSpinner(fmt.Sprintf("Stage 1/3: starting remote database dump '%s'...", cfg.Remote.DB))
	dump, err := provider.DumpRemote(ctx)
	if err != nil {
		spinner.Fail(fmt.Sprintf("Stage 1/3 failed: remote database dump: %v", err))
		return fmt.Errorf("failed to dump remote db: %w", err)
	}
	spinner.Success(fmt.Sprintf("Stage 1/3 complete: remote dump stream started for '%s'", cfg.Remote.DB))

	progress := &dbStreamProgress{}
	label := fmt.Sprintf("DB 2/3 transform (%s) + 3/3 import local '%s'", ReplacementOptionsFromConfig(cfg, cfg.DBReplace).Engine, cfg.Local.DB)
	spinner = startSpinner(label + "...")
	stopProgress := startDBProgress(ctx, spinner, label, progress)
	if err := writeTransformedDump(ctx, dump, cfg, cfg.DBReplace, dumpDB, "db.sql", provider.WriteLocal, progress); err != nil {
		stopProgress()
		spinner.Fail(fmt.Sprintf("Stage 2/3 + 3/3 failed: local database import: %v", err))
		return fmt.Errorf("failed to write to local db: %w", err)
	}
	stopProgress()
	spinner.Success(fmt.Sprintf("Stage 3/3 complete: wrote %s to local database '%s'", formatBytes(progress.outputBytes.Load()), cfg.Local.DB))

	return nil
}

func syncDBReverse(ctx context.Context, provider DBProvider, cfg *Config, dumpDB bool) error {
	pterm.DefaultSection.Println("Syncing Database (local to remote)")

	spinner := startSpinner(fmt.Sprintf("Stage 1/4: starting local database dump '%s'...", cfg.Local.DB))
	dump, err := provider.DumpLocal(ctx)
	if err != nil {
		spinner.Fail(fmt.Sprintf("Stage 1/4 failed: local database dump: %v", err))
		return fmt.Errorf("failed to dump local db: %w", err)
	}
	spinner.Success(fmt.Sprintf("Stage 1/4 complete: local dump stream started for '%s'", cfg.Local.DB))

	var reversedReplacements []DBReplace
	for i := len(cfg.DBReplace) - 1; i >= 0; i-- {
		r := cfg.DBReplace[i]
		reversedReplacements = append(reversedReplacements, DBReplace{From: r.To, To: r.From})
	}

	spinner = startSpinner("Stage 2/4: backing up remote database before import...")
	if err := provider.BackupRemote(ctx); err != nil {
		_ = dump.Reader.Close()
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

func writeTransformedDump(ctx context.Context, dump *DBDump, cfg *Config, replacements []DBReplace, dumpDB bool, dumpPath string, writeDB func(context.Context, io.Reader) error, progress *dbStreamProgress) error {
	defer dump.Reader.Close()

	inputReader := dump.Reader
	if progress != nil {
		inputReader = &countingReadCloser{ReadCloser: dump.Reader, counter: &progress.sourceBytes}
	}

	reader, transformErr := transformDumpAsync(inputReader, cfg, replacements)
	defer reader.Close()

	var input io.Reader = reader
	if progress != nil {
		input = &countingReader{reader: reader, counter: &progress.outputBytes}
	}
	var dumpFile *os.File
	if dumpDB {
		file, err := os.Create(dumpPath)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", dumpPath, err)
		}
		dumpFile = file
		input = io.TeeReader(input, dumpFile)
	}

	writeErr := writeDB(ctx, input)
	if writeErr != nil {
		_ = reader.Close()
	}

	if err := <-transformErr; err != nil {
		writeErr = err
	}
	if writeErr != nil {
		_ = dump.Reader.Close()
	}
	if err := dump.Wait(); err != nil && writeErr == nil {
		writeErr = err
	}
	if dumpFile != nil {
		if err := dumpFile.Close(); err != nil && writeErr == nil {
			writeErr = fmt.Errorf("failed to close %s: %w", dumpPath, err)
		}
	}
	return writeErr
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

func transformDumpAsync(input io.Reader, cfg *Config, replacements []DBReplace) (*io.PipeReader, <-chan error) {
	reader, writer := io.Pipe()
	errs := make(chan error, 1)

	go func() {
		err := TransformSQLDump(input, writer, ReplacementOptionsFromConfig(cfg, replacements))
		if closeErr := writer.CloseWithError(err); closeErr != nil && err == nil {
			err = closeErr
		}
		errs <- err
	}()

	return reader, errs
}

type dbStreamProgress struct {
	sourceBytes atomic.Int64
	outputBytes atomic.Int64
}

type countingReadCloser struct {
	io.ReadCloser
	counter *atomic.Int64
}

func (r *countingReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		r.counter.Add(int64(n))
	}
	return n, err
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
	args := []string{
		"-p", p.cfg.Port,
		p.cfg.SSHHost,
		"mysqldump", "-uroot",
	}
	args = append(args, mysqlDumpFlags()...)
	args = append(args, p.cfg.Remote.DB)

	cmd := exec.CommandContext(ctx, "ssh", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to open ssh stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
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
	composeFile := getComposeFilePath()
	remoteCommand := fmt.Sprintf(
		"if command -v mariadb-dump >/dev/null 2>&1; then mariadb-dump -uroot -psecret %s %s; else mysqldump -uroot -psecret %s %s; fi",
		strings.Join(mysqlDumpFlags(), " "), shellQuote(p.cfg.Local.DB), strings.Join(mysqlDumpFlags(), " "), shellQuote(p.cfg.Local.DB),
	)

	args := []string{
		"compose",
		"-f", composeFile,
		"exec", "-T",
		"mariadb", "sh", "-c", remoteCommand,
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to open docker stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
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
	args := []string{
		"-p", p.cfg.Port,
		p.cfg.SSHHost,
		"mysql", "-uroot", p.cfg.Remote.DB,
	}

	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdin = sqlDump
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

	args := []string{
		"compose",
		"-f", composeFile,
		"exec", "-T",
		"mariadb", "mariadb",
		"-uroot", "-psecret",
		p.cfg.Local.DB,
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = sqlDump
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker command failed: %s: %w", string(output), err)
	}

	return nil
}

func (p *RealDBProvider) BackupRemote(ctx context.Context) error {
	timestamp := time.Now().Format("20060102_150405")
	backupFile := fmt.Sprintf("%s_backup_%s.sql", p.cfg.Remote.DB, timestamp)
	remoteCmd := fmt.Sprintf("mysqldump -uroot %s %s > %s", strings.Join(mysqlDumpFlags(), " "), shellQuote(p.cfg.Remote.DB), shellQuote(backupFile))

	args := []string{
		"-p", p.cfg.Port,
		p.cfg.SSHHost,
		remoteCmd,
	}

	cmd := exec.CommandContext(ctx, "ssh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh backup command failed: %s: %w", string(output), err)
	}
	return nil
}

func ensureUserAndDB(ctx context.Context, dbName, composeFile string) error {
	query := fmt.Sprintf(
		"CREATE USER IF NOT EXISTS `%[1]s`@'%%' IDENTIFIED BY 'secret'; "+
			"CREATE DATABASE IF NOT EXISTS `%[1]s`; "+
			"GRANT ALL PRIVILEGES ON `%[1]s`.* TO `%[1]s`@'%%';",
		dbName,
	)

	args := []string{
		"compose",
		"-f", composeFile,
		"exec", "-T",
		"mariadb", "mariadb",
		"-uroot", "-psecret",
		"-e", query,
	}

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

func applyStringReplacements(value string, replacements []DBReplace) string {
	for _, item := range replacements {
		variations := []struct {
			from string
			to   string
		}{
			{strings.ReplaceAll(item.From, "/", `\/`), strings.ReplaceAll(item.To, "/", `\/`)},
			{strings.ReplaceAll(item.From, "/", `\\/`), strings.ReplaceAll(item.To, "/", `\\/`)},
			{strings.ReplaceAll(item.From, "/", `\\\/`), strings.ReplaceAll(item.To, "/", `\\\/`)},
			{strings.ReplaceAll(item.From, "/", `\\\\/`), strings.ReplaceAll(item.To, "/", `\\\\/`)},
			{strings.ReplaceAll(item.From, "/", `\\\\\/`), strings.ReplaceAll(item.To, "/", `\\\\\/`)},
		}

		for _, v := range variations {
			if v.from != item.From {
				value = strings.ReplaceAll(value, v.from, v.to)
			}
		}
		value = strings.ReplaceAll(value, item.From, item.To)
	}
	return value
}
