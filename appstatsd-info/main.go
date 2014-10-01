package main

import (
	"bytes"
	"code.google.com/p/plotinum/vg"
	"flag"
	"github.com/op/go-logging"
	"path"
	"strings"
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

	// manually load fonts from resource
	// ONLY IN CUSTOM REPOSITORY: https://code.google.com/r/rangelspam-plotium
	for _, fontasset := range AssetNames() {
		if strings.HasPrefix(fontasset, "res/fonts/") {
			fontname := strings.TrimSuffix(path.Base(fontasset), path.Ext(fontasset))
			fontbytes, err := Asset(fontasset)
			if err != nil {
				panic(err)
			}
			fontreader := bytes.NewReader(fontbytes)
			vg.LoadFont(fontname, fontreader)
		}
	}

	go ServerInfo()

	select {}
}
