package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type DBProvider interface {
	DumpRemote(ctx context.Context) (string, error)
	DumpLocal(ctx context.Context) (string, error)
	WriteRemote(ctx context.Context, sql string) error
	WriteLocal(ctx context.Context, sql string) error
	BackupRemote(ctx context.Context) error
}

type RealDBProvider struct {
	cfg *Config
}

func NewRealDBProvider(cfg *Config) *RealDBProvider {
	return &RealDBProvider{cfg: cfg}
}

func SyncDB(ctx context.Context, provider DBProvider, cfg *Config, dumpDB bool, reverse bool) error {
	if reverse {
		return syncDBReverse(ctx, provider, cfg, dumpDB)
	}

	// 1. Dump remote DB
	fmt.Printf("Dumping remote database '%s'...\n", cfg.Remote.DB)
	sqlDump, err := provider.DumpRemote(ctx)
	if err != nil {
		return fmt.Errorf("failed to dump remote db: %w", err)
	}

	// 2. Apply replacements
	fmt.Println("Applying replacements...")
	sqlDump = ApplyDBReplacements(sqlDump, cfg.DBReplace)

	// 3. Write to local DB
	fmt.Printf("Writing to local database '%s'...\n", cfg.Local.DB)
	if err := provider.WriteLocal(ctx, sqlDump); err != nil {
		return fmt.Errorf("failed to write to local db: %w", err)
	}

	if dumpDB {
		fmt.Println("Saving db.sql...")
		if err := os.WriteFile("db.sql", []byte(sqlDump), 0644); err != nil {
			return fmt.Errorf("failed to save db.sql: %w", err)
		}
	}

	return nil
}

func syncDBReverse(ctx context.Context, provider DBProvider, cfg *Config, dumpDB bool) error {
	// 1. Dump local DB
	fmt.Printf("Dumping local database '%s'...\n", cfg.Local.DB)
	sqlDump, err := provider.DumpLocal(ctx)
	if err != nil {
		return fmt.Errorf("failed to dump local db: %w", err)
	}

	// 2. Apply replacements (Reversed)
	fmt.Println("Applying replacements (Reverse)...")
	var reversedReplacements []DBReplace
	for _, r := range cfg.DBReplace {
		reversedReplacements = append(reversedReplacements, DBReplace{From: r.To, To: r.From})
	}
	sqlDump = ApplyDBReplacements(sqlDump, reversedReplacements)

	if dumpDB {
		fmt.Println("Saving db_reverse.sql...")
		if err := os.WriteFile("db_reverse.sql", []byte(sqlDump), 0644); err != nil {
			return fmt.Errorf("failed to save db_reverse.sql: %w", err)
		}
	}

	// 3. Backup Remote DB
	fmt.Println("Backing up remote database...")
	if err := provider.BackupRemote(ctx); err != nil {
		return fmt.Errorf("failed to backup remote db: %w", err)
	}

	// 4. Write to remote DB
	fmt.Printf("Writing to remote database '%s'...\n", cfg.Remote.DB)
	if err := provider.WriteRemote(ctx, sqlDump); err != nil {
		return fmt.Errorf("failed to write to remote db: %w", err)
	}

	return nil
}

func (p *RealDBProvider) DumpRemote(ctx context.Context) (string, error) {
	args := []string{
		p.cfg.SSHHost,
		"-p", p.cfg.Port,
		"mysqldump", "-uroot", p.cfg.Remote.DB,
	}

	cmd := exec.CommandContext(ctx, "ssh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ssh command failed: %s: %w", stderr.String(), err)
	}

	return stdout.String(), nil
}

func (p *RealDBProvider) DumpLocal(ctx context.Context) (string, error) {
	composeFile := getComposeFilePath()

	// Try mariadb-dump first (modern MariaDB containers)
	args := []string{
		"compose",
		"-f", composeFile,
		"exec", "-T",
		"mariadb", "mariadb-dump",
		"-uroot", "-psecret",
		p.cfg.Local.DB,
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err == nil {
		return stdout.String(), nil
	}

	// Fallback to mysqldump (older containers or MySQL)
	args[6] = "mysqldump"
	cmd = exec.CommandContext(ctx, "docker", args...)
	stdout.Reset()
	stderr.Reset()
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker command failed (stderr: %s): %w", stderr.String(), err)
	}

	return stdout.String(), nil
}

func (p *RealDBProvider) WriteRemote(ctx context.Context, sqlDump string) error {
	args := []string{
		p.cfg.SSHHost,
		"-p", p.cfg.Port,
		"mysql", "-uroot", p.cfg.Remote.DB,
	}

	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdin = strings.NewReader(sqlDump)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh command failed: %w", err)
	}

	return nil
}

func (p *RealDBProvider) WriteLocal(ctx context.Context, sqlDump string) error {
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
	cmd.Stdin = strings.NewReader(sqlDump)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker command failed: %w", err)
	}

	return nil
}

func (p *RealDBProvider) BackupRemote(ctx context.Context) error {
	timestamp := time.Now().Format("20060102_150405")
	backupFile := fmt.Sprintf("%s_backup_%s.sql", p.cfg.Remote.DB, timestamp)

	// Command: mysqldump -uroot dbname > backup_file.sql
	remoteCmd := fmt.Sprintf("mysqldump -uroot %s > %s", p.cfg.Remote.DB, backupFile)

	args := []string{
		p.cfg.SSHHost,
		"-p", p.cfg.Port,
		remoteCmd,
	}

	fmt.Printf("Creating remote backup: %s\n", backupFile)
	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh backup command failed: %w", err)
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create user/db: %w", err)
	}
	return nil
}

func getComposeFilePath() string {
	// Preserve original behavior but allow override
	if path := os.Getenv("DSYNC_COMPOSE_FILE"); path != "" {
		return path
	}
	return os.Getenv("HOME") + "/www/dev/docker-compose.yml"
}

func ApplyDBReplacements(sql string, replacements []DBReplace) string {
	maxLen := 0
	for _, item := range replacements {
		if len(item.From) > maxLen {
			maxLen = len(item.From)
		}
	}

	for _, item := range replacements {
		fmt.Printf("%-*s -> %s\n", maxLen, item.From, item.To)
		sql = strings.ReplaceAll(sql, item.From, item.To)

		// Handle JSON-escaped slashes (e.g. "http:\/\/")
		// Common in JSON embedded in HTML or PHP serialized data
		fromJSON := strings.ReplaceAll(item.From, "/", `\/`)
		toJSON := strings.ReplaceAll(item.To, "/", `\/`)
		if fromJSON != item.From {
			sql = strings.ReplaceAll(sql, fromJSON, toJSON)
		}

		// Handle Double-escaped slashes (e.g. "http:\\/\\/")
		// Common in SQL dumps of JSON data where backslashes are escaped
		fromDouble := strings.ReplaceAll(item.From, "/", `\\/`)
		toDouble := strings.ReplaceAll(item.To, "/", `\\/`)
		if fromDouble != item.From {
			sql = strings.ReplaceAll(sql, fromDouble, toDouble)
		}
	}
	return sql
}
