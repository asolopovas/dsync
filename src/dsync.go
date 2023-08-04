package dsync

import (
	"flag"
	"fmt"
	"os"
)

func Run() {
	var dump string
	var aFlag = flag.Bool("a", false, "Sync Files and Database")
	var cFlag = flag.String("c", "dsync-config.json", "Custom config path")
	var fFlag = flag.Bool("f", false, "Sync Files only")
	var dFlag = flag.Bool("d", false, "Sync Database only")
	var gFlag = flag.Bool("g", false, "Generate default config")
	var dumpFlag = flag.Bool("dump", false, "Dump Database to file")
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
			dump, err = SyncDb(conf)
			ErrChk(err)
			WriteToLocalDB(dump, conf, *dumpFlag)
		}
	}
}
