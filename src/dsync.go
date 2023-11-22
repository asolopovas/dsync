package dsync

import (
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

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
			&cli.StringFlag{
				Name:  "c",
				Value: "dsync-config.json",
				Usage: "Custom config path",
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
				fmt.Println("Syncing Files and Database")
				SyncFiles(conf)
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
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

// Define other functions like GenConfig, GetJsonConfig, SyncFiles, SyncDb, WriteToLocalDB, etc.
