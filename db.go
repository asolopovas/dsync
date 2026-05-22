package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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

	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Dumping remote database '%s'...", cfg.Remote.DB))
	dump, err := provider.DumpRemote(ctx)
	if err != nil {
		spinner.Fail(fmt.Sprintf("Failed to dump remote db: %v", err))
		return fmt.Errorf("failed to dump remote db: %w", err)
	}
	spinner.Success(fmt.Sprintf("Started remote database dump '%s'", cfg.Remote.DB))

	spinner, _ = pterm.DefaultSpinner.Start("Applying replacements and writing to local database...")
	if err := writeTransformedDump(ctx, dump, cfg, cfg.DBReplace, dumpDB, "db.sql", provider.WriteLocal); err != nil {
		spinner.Fail(fmt.Sprintf("Failed to write to local db: %v", err))
		return fmt.Errorf("failed to write to local db: %w", err)
	}
	spinner.Success(fmt.Sprintf("Wrote to local database '%s'", cfg.Local.DB))

	return nil
}

func syncDBReverse(ctx context.Context, provider DBProvider, cfg *Config, dumpDB bool) error {
	pterm.DefaultSection.Println("Syncing Database (local to remote)")

	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Dumping local database '%s'...", cfg.Local.DB))
	dump, err := provider.DumpLocal(ctx)
	if err != nil {
		spinner.Fail(fmt.Sprintf("Failed to dump local db: %v", err))
		return fmt.Errorf("failed to dump local db: %w", err)
	}
	spinner.Success(fmt.Sprintf("Started local database dump '%s'", cfg.Local.DB))

	var reversedReplacements []DBReplace
	for i := len(cfg.DBReplace) - 1; i >= 0; i-- {
		r := cfg.DBReplace[i]
		reversedReplacements = append(reversedReplacements, DBReplace{From: r.To, To: r.From})
	}

	spinner, _ = pterm.DefaultSpinner.Start("Backing up remote database...")
	if err := provider.BackupRemote(ctx); err != nil {
		_ = dump.Reader.Close()
		spinner.Fail(fmt.Sprintf("Failed to backup remote db: %v", err))
		return fmt.Errorf("failed to backup remote db: %w", err)
	}
	spinner.Success("Backed up remote database")

	spinner, _ = pterm.DefaultSpinner.Start("Applying replacements (Reverse) and writing to remote database...")
	if err := writeTransformedDump(ctx, dump, cfg, reversedReplacements, dumpDB, "db_reverse.sql", provider.WriteRemote); err != nil {
		spinner.Fail(fmt.Sprintf("Failed to write to remote db: %v", err))
		return fmt.Errorf("failed to write to remote db: %w", err)
	}
	spinner.Success(fmt.Sprintf("Wrote to remote database '%s'", cfg.Remote.DB))

	return nil
}

func writeTransformedDump(ctx context.Context, dump *DBDump, cfg *Config, replacements []DBReplace, dumpDB bool, dumpPath string, writeDB func(context.Context, io.Reader) error) error {
	defer dump.Reader.Close()

	reader, transformErr := transformDumpAsync(dump.Reader, cfg, replacements)
	defer reader.Close()

	var input io.Reader = reader
	var dumpFile *os.File
	if dumpDB {
		file, err := os.Create(dumpPath)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", dumpPath, err)
		}
		dumpFile = file
		input = io.TeeReader(reader, dumpFile)
	}

	writeErr := writeDB(ctx, input)
	if writeErr != nil {
		_ = reader.Close()
	}

	if err := <-transformErr; err != nil && writeErr == nil {
		writeErr = err
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
