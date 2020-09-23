package main

import (
	"flag"

	"github.com/bio-routing/flowhouse/cmd/flowhouse/config"
	"github.com/bio-routing/flowhouse/pkg/flowhouse"

	log "github.com/sirupsen/logrus"
)

var (
	configFilePath = flag.String("config.file", "config.yaml", "Config file path (YAML)")
)

func main() {
	flag.Parse()

	cfg, err := config.GetConfig(*configFilePath)
	if err != nil {
		log.WithError(err).Fatal("Unable to get config")
	}

	fh := flowhouse.New(cfg)
	fh.Start()

	select {}
}
