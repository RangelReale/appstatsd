package main

import (
	"flag"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("appstatsd-info")

var configfile = flag.String("configfile", "", "configuration file path")

func main() {
	flag.Parse()

	// load configuration file
	if *configfile != "" {
		log.Debug("Loading configuration file %s", *configfile)

		err := Configuration.Load(*configfile)
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	go ServerInfo()

	select {}
}
