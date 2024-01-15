package dsync

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func Run() {
	if err := newRootCmd().Execute(); err != nil {
		log.Fatal(err)
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

	var rootCmd = &cobra.Command{
		Use:   "dsync",
		Short: "A tool to sync files and databases between different environments version: v1.0.6",
		Run: func(cmd *cobra.Command, args []string) {
			flagSet := syncFilesAndDB || syncFilesOnly || syncDBOnly || dumpDB || generateConfig || showVersion

			if len(args) == 0 && !flagSet {
				cmd.Help()
				return
			}

			if showVersion {
				fmt.Println("v1.0.6")
				return
			}

			if generateConfig {
				GenConfig()
				return
			}

			conf, err := GetJsonConfig(configPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading config file '%s': %v\n", configPath, err)
				cmd.Help() // Display help message
				return
			}

			if syncFilesAndDB {
				SyncFiles(conf)
				WriteRemoteToLocalDb(conf, dumpDB)
			}

			if syncFilesOnly {
				SyncFiles(conf)
			}

			if syncDBOnly {
				WriteRemoteToLocalDb(conf, dumpDB)
			}

		},
	}

	rootCmd.Flags().BoolVarP(&syncFilesAndDB, "a", "a", false, "Sync Files and Database")
	rootCmd.Flags().BoolVarP(&syncFilesOnly, "f", "f", false, "Sync Files only")
	rootCmd.Flags().BoolVarP(&syncDBOnly, "d", "d", false, "Sync Database only")
	rootCmd.Flags().BoolVarP(&dumpDB, "dump", "", false, "Dump Database to file") // Changed here
	rootCmd.Flags().BoolVarP(&generateConfig, "g", "g", false, "Generate default config")
	rootCmd.Flags().BoolVarP(&showVersion, "v", "v", false, "Get Version")
	rootCmd.Flags().StringVarP(&configPath, "c", "c", "dsync-config.json", "Custom config path")

	rootCmd.AddCommand(newCompletionCmd())

	return rootCmd
}

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion",
		Short: "Generate fish completion script",
		Run:   generateFishCompletion,
	}
}

func generateFishCompletion(cmd *cobra.Command, args []string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to get user home directory: %v", err)
	}

	fishCompletionDir := filepath.Join(homeDir, ".config", "fish", "completions")
	if err := os.MkdirAll(fishCompletionDir, os.ModePerm); err != nil {
		log.Fatalf("failed to create fish completions directory: %v", err)
	}

	fishCompletionFile := filepath.Join(fishCompletionDir, "dsync.fish")
	f, err := os.Create(fishCompletionFile)
	if err != nil {
		log.Fatalf("failed to create fish completion file: %v", err)
	}
	defer f.Close()

	if err := cmd.Root().GenFishCompletion(f, true); err != nil {
		log.Fatalf("failed to generate fish completion script: %v", err)
	}

	fmt.Printf("Fish completion script generated at: %s\n", fishCompletionFile)
}
