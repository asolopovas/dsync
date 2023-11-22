package dsync

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"
)

func generateFishCompletion(c *cli.Context) error {
	script, err := c.App.ToFishCompletion()
	if err != nil {
		return err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	fishCompletionDir := filepath.Join(homeDir, ".config", "fish", "completions")
	if err := os.MkdirAll(fishCompletionDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create fish completions directory: %w", err)
	}

	fishCompletionFile := filepath.Join(fishCompletionDir, "dsync.fish")
	if err := os.WriteFile(fishCompletionFile, []byte(script), 0644); err != nil {
		return fmt.Errorf("failed to write fish completion file: %w", err)
	}

	fmt.Printf("Fish completion script generated at: %s\n", fishCompletionFile)
	return nil
}

func Run() {
	app := &cli.App{
		Name:                 "dsync",
		Usage:                "A tool to sync files and databases between different environments",
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "a",
				Usage: "Sync Files and Database",
			},
			&cli.BoolFlag{
				Name:  "f",
				Usage: "Sync Files only",
			},
			&cli.BoolFlag{
				Name:  "d",
				Usage: "Sync Database only",
			},
			&cli.BoolFlag{
				Name:  "dump",
				Usage: "Dump Database to file",
			},
			&cli.BoolFlag{
				Name:  "g",
				Usage: "Generate default config",
			},
			&cli.BoolFlag{
				Name:  "v",
				Usage: "Get Version",
			},

			&cli.StringFlag{
				Name:  "c",
				Value: "dsync-config.json",
				Usage: "Custom config path",
			},
		},
		Action: func(c *cli.Context) error {
			if c.NumFlags() == 0 {
				cli.ShowAppHelp(c)
				return nil
			}

			if c.Bool("v") {
				fmt.Println("v1.0.5")
				return nil
			}

			if c.Bool("g") {
				GenConfig()
				return nil
			}

			conf, err := GetJsonConfig(c.String("c"))
			if err != nil {
				fmt.Printf("%s config could not be loaded\n", c.String("c"))
				return err
			}

			if c.Bool("a") {
				fmt.Println("Syncing Files")
				SyncFiles(conf)
				fmt.Println("Syncing Database")
				SyncDb(conf)
			} else {
				if c.Bool("f") {
					fmt.Println("Syncing Files")
					SyncFiles(conf)
				}

				if c.Bool("d") {
					fmt.Println("Syncing Database")
					dump, err := SyncDb(conf)
					if err != nil {
						return err
					}
					WriteToLocalDB(dump, conf, c.Bool("dump"))
				}
			}
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:   "completion",
				Usage:  "Generate fish completion script",
				Action: generateFishCompletion,
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
