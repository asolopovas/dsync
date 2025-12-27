package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed version
var version string

func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var (
		syncFilesAndDB bool
		syncFilesOnly  bool
		syncDBOnly     bool
		dumpDB         bool
		generateConfig bool
		showVersion    bool
		configPath     string
	)

	rootCmd := &cobra.Command{
		Use:   "dsync",
		Short: fmt.Sprintf("A tool to sync files and databases between different environments version: %s", strings.TrimSpace(version)),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if any flag is set
			flagSet := syncFilesAndDB || syncFilesOnly || syncDBOnly || dumpDB || generateConfig || showVersion

			if len(args) == 0 && !flagSet {
				return cmd.Help()
			}

			if showVersion {
				fmt.Println(strings.TrimSpace(version))
				return nil
			}

			if generateConfig {
				return GenerateConfig("dsync-config.json")
			}

			cfg, err := LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("error loading config file '%s': %w", configPath, err)
			}

			ctx := context.Background()

			if syncFilesAndDB || syncFilesOnly {
				if err := SyncFiles(ctx, cfg); err != nil {
					return err
				}
			}

			if syncFilesAndDB || syncDBOnly {
				if err := SyncDB(ctx, cfg, dumpDB); err != nil {
					return err
				}
			}

			return nil
		},
	}

	rootCmd.Flags().BoolVarP(&syncFilesAndDB, "all", "a", false, "Sync Files and Database")
	rootCmd.Flags().BoolVarP(&syncFilesOnly, "files", "f", false, "Sync Files only")
	rootCmd.Flags().BoolVarP(&syncDBOnly, "db", "d", false, "Sync Database only")
	rootCmd.Flags().BoolVarP(&dumpDB, "dump", "", false, "Dump Database to file")
	rootCmd.Flags().BoolVarP(&generateConfig, "gen", "g", false, "Generate default config")
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Get Version")
	rootCmd.Flags().StringVarP(&configPath, "config", "c", "dsync-config.json", "Custom config path")

	rootCmd.AddCommand(newCompletionCmd())

	return rootCmd
}

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion",
		Short: "Generate fish completion script",
		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get user home directory: %w", err)
			}

			fishCompletionDir := filepath.Join(homeDir, ".config", "fish", "completions")
			if err := os.MkdirAll(fishCompletionDir, 0755); err != nil {
				return fmt.Errorf("failed to create completion directory: %w", err)
			}

			filePath := filepath.Join(fishCompletionDir, "dsync.fish")
			if err := cmd.Root().GenFishCompletionFile(filePath, true); err != nil {
				return fmt.Errorf("failed to generate fish completion: %w", err)
			}

			fmt.Printf("Fish completion generated at: %s\n", filePath)
			return nil
		},
	}
}
