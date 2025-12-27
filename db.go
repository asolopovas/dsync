package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func SyncDB(ctx context.Context, cfg *Config, dumpDB bool) error {
	// 1. Dump remote DB
	fmt.Printf("Dumping remote database '%s'...\n", cfg.Remote.DB)
	sqlDump, err := getRemoteSQLDump(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to dump remote db: %w", err)
	}

	// 2. Apply replacements
	fmt.Println("Applying replacements...")
	sqlDump = ApplyDBReplacements(sqlDump, cfg.DBReplace)

	// 3. Write to local DB
	fmt.Printf("Writing to local database '%s'...\n", cfg.Local.DB)
	if err := writeToLocalDB(ctx, cfg, sqlDump, dumpDB); err != nil {
		return fmt.Errorf("failed to write to local db: %w", err)
	}

	return nil
}

func getRemoteSQLDump(ctx context.Context, cfg *Config) (string, error) {
	args := []string{
		cfg.SSHHost,
		"-p", cfg.Port,
		"mysqldump", "-uroot", cfg.Remote.DB,
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

func writeToLocalDB(ctx context.Context, cfg *Config, sqlDump string, dumpToFile bool) error {
	composeFile := getComposeFilePath()

	if err := ensureUserAndDB(ctx, cfg.Local.DB, composeFile); err != nil {
		return err
	}

	if dumpToFile {
		fmt.Println("Saving db.sql...")
		if err := os.WriteFile("db.sql", []byte(sqlDump), 0644); err != nil {
			return fmt.Errorf("failed to save db.sql: %w", err)
		}
	}

	args := []string{
		"compose",
		"-f", composeFile,
		"exec", "-T",
		"mariadb", "mariadb",
		"-uroot", "-psecret",
		cfg.Local.DB,
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
	}
	return sql
}
