package dsync

import (
	"flag"
	"fmt"
	"os"
)

func Run() {
	var aFlag = flag.Bool("a", false, "Sync Files and DB")
	var cFlag = flag.String("c", "dsync-config.json", "custom config path")
	var fFlag = flag.Bool("f", false, "Sync files")
	var dFlag = flag.Bool("d", false, "Sync Db")
	var gFlag = flag.Bool("g", false, "Generate default config")
	flag.Parse()

	if len(os.Args) == 1 {
		flag.PrintDefaults()
		return
	}

	if *gFlag {
		GenConfig()
		return
	}

	conf, err := GetJsonConfig(*cFlag)
	if err != nil {
		fmt.Println(*cFlag + " config could not be loaded")
		return
	}

	if *aFlag {
		SyncFiles(conf)
		SyncDb(conf)
	} else {
		if *fFlag {
			SyncFiles(conf)
		}

		if *dFlag {
			SyncDb(conf)
		}
	}
}
